package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/mstfsu/go-case/internal/domain"
)

type NotificationRepository interface {
	Create(ctx context.Context, n *domain.Notification) error
	CreateBatch(ctx context.Context, notifications []*domain.Notification) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, taskID string) error
	UpdateDelivered(ctx context.Context, id uuid.UUID, providerMessageID string) error
	UpdateFailed(ctx context.Context, id uuid.UUID, errorMessage string) error
	IncrementRetry(ctx context.Context, id uuid.UUID) error
	Cancel(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, filter domain.ListFilter) (*domain.ListResult, error)
	GetBatchStatus(ctx context.Context, batchID uuid.UUID) (*domain.BatchStatus, error)
	LogAttempt(ctx context.Context, attempt *domain.NotificationAttempt) error
	GetAttempts(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationAttempt, error)
}
