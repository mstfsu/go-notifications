package worker

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
)

func NewServer(redisOpt asynq.RedisClientOpt, concurrency int, logger zerolog.Logger) *asynq.Server {
	return asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues: map[string]int{
			QueueCritical: 6,
			QueueDefault:  3,
			QueueLow:      1,
		},
		RetryDelayFunc: retryDelay,
		ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
			logger.Error().
				Str("task_type", task.Type()).
				Err(err).
				Msg("task error handler triggered")
		}),
		Logger: &asynqLogger{logger: logger},
	})
}

func retryDelay(n int, _ error, _ *asynq.Task) time.Duration {
	base := time.Duration(math.Pow(2, float64(n))) * 10 * time.Second
	if base > time.Hour {
		base = time.Hour
	}
	jitter := time.Duration(rand.Intn(10)) * time.Second
	return base + jitter
}

type asynqLogger struct {
	logger zerolog.Logger
}

func (l *asynqLogger) Debug(args ...any) {
	l.logger.Debug().Msgf("%v", args)
}
func (l *asynqLogger) Info(args ...any) {
	l.logger.Info().Msgf("%v", args)
}
func (l *asynqLogger) Warn(args ...any) {
	l.logger.Warn().Msgf("%v", args)
}
func (l *asynqLogger) Error(args ...any) {
	l.logger.Error().Msgf("%v", args)
}
func (l *asynqLogger) Fatal(args ...any) {
	l.logger.Fatal().Msgf("%v", args)
}
