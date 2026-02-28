package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"golang.org/x/time/rate"

	"github.com/mstfsu/go-case/internal/domain"
	"github.com/mstfsu/go-case/internal/provider"
	"github.com/mstfsu/go-case/internal/repository"
)

type rateLimiters struct {
	mu       sync.Mutex
	limiters map[domain.Channel]*rate.Limiter
}

func newRateLimiters() *rateLimiters {
	return &rateLimiters{
		limiters: make(map[domain.Channel]*rate.Limiter),
	}
}

func (rl *rateLimiters) get(ch domain.Channel) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok := rl.limiters[ch]; ok {
		return l
	}
	// 100 messages per second, burst of 100
	l := rate.NewLimiter(rate.Limit(100), 100)
	rl.limiters[ch] = l
	return l
}

type Processor struct {
	repo     repository.NotificationRepository
	provider *provider.WebhookProvider
	limiters *rateLimiters
	logger   zerolog.Logger
}

func NewProcessor(
	repo repository.NotificationRepository,
	prov *provider.WebhookProvider,
	logger zerolog.Logger,
) *Processor {
	return &Processor{
		repo:     repo,
		provider: prov,
		limiters: newRateLimiters(),
		logger:   logger,
	}
}

func (p *Processor) HandleSendNotification(ctx context.Context, t *asynq.Task) error {
	var payload NotificationPayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	notifID, err := uuid.Parse(payload.NotificationID)
	if err != nil {
		return fmt.Errorf("invalid notification id: %w", err)
	}

	n, err := p.repo.GetByID(ctx, notifID)
	if err != nil {
		return fmt.Errorf("get notification: %w", err)
	}
	if n == nil {
		p.logger.Warn().Str("id", notifID.String()).Msg("notification not found, skipping")
		return nil
	}

	if n.Status == domain.StatusCancelled {
		p.logger.Info().Str("id", notifID.String()).Msg("notification cancelled, skipping")
		return nil
	}

	if err := p.repo.UpdateStatus(ctx, notifID, domain.StatusProcessing, t.ResultWriter().TaskID()); err != nil {
		return fmt.Errorf("update status to processing: %w", err)
	}

	limiter := p.limiters.get(n.Channel)
	if err := limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	attemptNum := n.RetryCount + 1

	sendResp, sendErr := p.provider.Send(ctx, n)

	attempt := &domain.NotificationAttempt{
		ID:             uuid.New(),
		NotificationID: notifID,
		AttemptNumber:  attemptNum,
		AttemptedAt:    time.Now().UTC(),
	}

	if sendErr != nil {
		attempt.Status = "failed"
		attempt.ErrorMessage = sendErr.Error()
		_ = p.repo.LogAttempt(ctx, attempt)
		_ = p.repo.IncrementRetry(ctx, notifID)

		retryInfo, _ := asynq.GetRetryCount(ctx)
		maxRetry, _ := asynq.GetMaxRetry(ctx)
		if retryInfo >= maxRetry {
			_ = p.repo.UpdateFailed(ctx, notifID, sendErr.Error())
			p.logger.Error().
				Str("id", notifID.String()).
				Int("attempt", attemptNum).
				Msg("notification permanently failed after max retries")
		} else {
			p.logger.Warn().
				Str("id", notifID.String()).
				Int("attempt", attemptNum).
				Err(sendErr).
				Msg("notification delivery failed, will retry")
		}
		return fmt.Errorf("provider send: %w", sendErr)
	}

	// Success
	attempt.Status = "success"
	attempt.ProviderResponse = map[string]any{
		"messageId": sendResp.MessageID,
		"status":    sendResp.Status,
		"timestamp": sendResp.Timestamp,
	}
	_ = p.repo.LogAttempt(ctx, attempt)

	if err := p.repo.UpdateDelivered(ctx, notifID, sendResp.MessageID); err != nil {
		return fmt.Errorf("update delivered: %w", err)
	}

	p.logger.Info().
		Str("id", notifID.String()).
		Str("provider_message_id", sendResp.MessageID).
		Msg("notification delivered successfully")

	return nil
}
