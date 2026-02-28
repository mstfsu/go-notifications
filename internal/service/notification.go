package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/mstfsu/go-case/internal/domain"
	"github.com/mstfsu/go-case/internal/repository"
	"github.com/mstfsu/go-case/internal/worker"
)

var (
	ErrNotFound         = errors.New("notification not found")
	ErrAlreadyExists    = errors.New("notification already exists (idempotent)")
	ErrCannotCancel     = errors.New("notification cannot be cancelled")
	ErrInvalidChannel   = errors.New("invalid channel")
	ErrInvalidPriority  = errors.New("invalid priority")
	ErrContentTooLong   = errors.New("content exceeds channel limit")
	ErrInvalidRecipient = errors.New("invalid recipient format")
	ErrBatchTooLarge    = errors.New("batch exceeds 1000 notifications")
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{7,14}$`)
)

const (
	maxSMSLength   = 160
	maxEmailLength = 10000
	maxPushLength  = 512
	maxBatchSize   = 1000
)

type NotificationService struct {
	repo  repository.NotificationRepository
	queue worker.Queue
}

func NewNotificationService(repo repository.NotificationRepository, queue worker.Queue) *NotificationService {
	return &NotificationService{repo: repo, queue: queue}
}

func (s *NotificationService) Create(ctx context.Context, req *domain.CreateRequest) (*domain.Notification, bool, error) {
	if err := s.validate(req); err != nil {
		return nil, false, err
	}

	idempotencyKey := buildIdempotencyKey(req)
	existing, err := s.repo.GetByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return nil, false, fmt.Errorf("check idempotency: %w", err)
	}
	if existing != nil {
		return existing, true, nil
	}

	n := &domain.Notification{
		ID:             uuid.New(),
		Recipient:      req.Recipient,
		Channel:        req.Channel,
		Content:        req.Content,
		Priority:       req.Priority,
		Status:         domain.StatusPending,
		IdempotencyKey: idempotencyKey,
		ScheduledAt:    req.ScheduledAt,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, n); err != nil {
		return nil, false, fmt.Errorf("create notification: %w", err)
	}
	if err := s.queue.Enqueue(ctx, n.ID.String(), n.Priority, n.ScheduledAt); err != nil {
		return nil, false, fmt.Errorf("enqueue notification: %w", err)
	}
	zerolog.Ctx(ctx).Info().
		Str("notification_id", n.ID.String()).
		Str("channel", string(n.Channel)).
		Str("priority", string(n.Priority)).
		Msg("notification created and enqueued")
	return n, false, nil
}

func (s *NotificationService) CreateBatch(ctx context.Context, reqs []*domain.CreateRequest) ([]*domain.Notification, error) {
	if len(reqs) > maxBatchSize {
		return nil, ErrBatchTooLarge
	}

	batchID := uuid.New()
	now := time.Now().UTC()
	notifications := make([]*domain.Notification, 0, len(reqs))

	for _, req := range reqs {
		if err := s.validate(req); err != nil {
			return nil, fmt.Errorf("validation failed for request: %w", err)
		}
		batchIDCopy := batchID
		notifications = append(notifications, &domain.Notification{
			ID:             uuid.New(),
			BatchID:        &batchIDCopy,
			Recipient:      req.Recipient,
			Channel:        req.Channel,
			Content:        req.Content,
			Priority:       req.Priority,
			Status:         domain.StatusPending,
			IdempotencyKey: buildIdempotencyKey(req),
			ScheduledAt:    req.ScheduledAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		})
	}

	if err := s.repo.CreateBatch(ctx, notifications); err != nil {
		return nil, fmt.Errorf("create batch: %w", err)
	}
	for _, n := range notifications {
		if err := s.queue.Enqueue(ctx, n.ID.String(), n.Priority, n.ScheduledAt); err != nil {
			return nil, fmt.Errorf("enqueue notification %s: %w", n.ID, err)
		}
	}
	zerolog.Ctx(ctx).Info().
		Str("batch_id", batchID.String()).
		Int("count", len(notifications)).
		Msg("batch created and enqueued")
	return notifications, nil
}

func (s *NotificationService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	n, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get by id: %w", err)
	}
	if n == nil {
		return nil, ErrNotFound
	}
	return n, nil
}

func (s *NotificationService) Cancel(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Cancel(ctx, id); err != nil {
		if err.Error() == "notification not found or not in pending state" {
			return ErrCannotCancel
		}
		return fmt.Errorf("cancel: %w", err)
	}
	zerolog.Ctx(ctx).Info().Str("notification_id", id.String()).Msg("notification cancelled")
	return nil
}

func (s *NotificationService) List(ctx context.Context, filter domain.ListFilter) (*domain.ListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.Limit < 1 || filter.Limit > 100 {
		filter.Limit = 20
	}
	return s.repo.List(ctx, filter)
}

func (s *NotificationService) GetBatchStatus(ctx context.Context, batchID uuid.UUID) (*domain.BatchStatus, error) {
	bs, err := s.repo.GetBatchStatus(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("get batch status: %w", err)
	}
	if bs.Total == 0 {
		return nil, ErrNotFound
	}
	return bs, nil
}

func (s *NotificationService) validate(req *domain.CreateRequest) error {
	if !req.Channel.IsValid() {
		return ErrInvalidChannel
	}
	if req.Priority == "" {
		req.Priority = domain.PriorityNormal
	}
	if !req.Priority.IsValid() {
		return ErrInvalidPriority
	}
	if err := validateRecipient(req.Channel, req.Recipient); err != nil {
		return err
	}
	if err := validateContent(req.Channel, req.Content); err != nil {
		return err
	}
	if req.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
	}
	return nil
}

func validateRecipient(ch domain.Channel, recipient string) error {
	switch ch {
	case domain.ChannelSMS:
		if !phoneRegex.MatchString(recipient) {
			return fmt.Errorf("%w: SMS recipient must be a valid phone number", ErrInvalidRecipient)
		}
	case domain.ChannelEmail:
		if !emailRegex.MatchString(recipient) {
			return fmt.Errorf("%w: email recipient must be a valid email address", ErrInvalidRecipient)
		}
	}
	return nil
}

func validateContent(ch domain.Channel, content string) error {
	limit := maxEmailLength
	switch ch {
	case domain.ChannelSMS:
		limit = maxSMSLength
	case domain.ChannelPush:
		limit = maxPushLength
	}
	if len([]rune(content)) > limit {
		return fmt.Errorf("%w: %s limit is %d characters", ErrContentTooLong, ch, limit)
	}
	return nil
}

func buildIdempotencyKey(req *domain.CreateRequest) string {
	h := sha256.New()
	h.Write([]byte(req.Recipient + "|" + string(req.Channel) + "|" + req.Content + "|" + req.IdempotencyKey))
	return fmt.Sprintf("%x", h.Sum(nil))
}
