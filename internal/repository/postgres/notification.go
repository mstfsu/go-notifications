package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mstfsu/go-case/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotificationRepo struct {
	db *gorm.DB
}

func NewNotificationRepo(db *gorm.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(ctx context.Context, n *domain.Notification) error {
	return r.db.WithContext(ctx).Create(toModel(n)).Error
}

func (r *NotificationRepo) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	models := make([]*notificationModel, len(notifications))
	for i, n := range notifications {
		models[i] = toModel(n)
	}
	return r.db.WithContext(ctx).CreateInBatches(models, 100).Error
}

func (r *NotificationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	var m notificationModel
	err := r.db.WithContext(ctx).First(&m, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *NotificationRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	var m notificationModel
	err := r.db.WithContext(ctx).Where("idempotency_key = ?", key).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toDomain(&m), nil
}

func (r *NotificationRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, taskID string) error {
	return r.db.WithContext(ctx).
		Model(&notificationModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": string(status), "task_id": taskID}).Error
}

func (r *NotificationRepo) UpdateDelivered(ctx context.Context, id uuid.UUID, providerMessageID string) error {
	return r.db.WithContext(ctx).
		Model(&notificationModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": string(domain.StatusDelivered), "provider_message_id": providerMessageID}).Error
}

func (r *NotificationRepo) UpdateFailed(ctx context.Context, id uuid.UUID, errorMessage string) error {
	return r.db.WithContext(ctx).
		Model(&notificationModel{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": string(domain.StatusFailed), "error_message": errorMessage}).Error
}

func (r *NotificationRepo) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&notificationModel{}).
		Where("id = ?", id).
		UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (r *NotificationRepo) Cancel(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Model(&notificationModel{}).
		Where("id = ? AND status = ?", id, domain.StatusPending).
		Update("status", string(domain.StatusCancelled))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("notification not found or not in pending state")
	}
	return nil
}

func (r *NotificationRepo) List(ctx context.Context, filter domain.ListFilter) (*domain.ListResult, error) {
	query := r.db.WithContext(ctx).Model(&notificationModel{})

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.Channel != "" {
		query = query.Where("channel = ?", filter.Channel)
	}
	if filter.DateFrom != nil {
		query = query.Where("created_at >= ?", filter.DateFrom)
	}
	if filter.DateTo != nil {
		query = query.Where("created_at <= ?", filter.DateTo)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var models []notificationModel
	offset := (filter.Page - 1) * filter.Limit
	if err := query.Order("created_at DESC").Limit(filter.Limit).Offset(offset).Find(&models).Error; err != nil {
		return nil, err
	}

	notifications := make([]*domain.Notification, len(models))
	for i := range models {
		notifications[i] = toDomain(&models[i])
	}

	return &domain.ListResult{
		Notifications: notifications,
		Total:         total,
		Page:          filter.Page,
		Limit:         filter.Limit,
	}, nil
}

func (r *NotificationRepo) GetBatchStatus(ctx context.Context, batchID uuid.UUID) (*domain.BatchStatus, error) {
	type batchStats struct {
		Total      int64
		Pending    int64
		Processing int64
		Delivered  int64
		Failed     int64
		Cancelled  int64
	}

	var s batchStats
	err := r.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*)                                      AS total,
			COUNT(*) FILTER (WHERE status = 'pending')    AS pending,
			COUNT(*) FILTER (WHERE status = 'processing') AS processing,
			COUNT(*) FILTER (WHERE status = 'delivered')  AS delivered,
			COUNT(*) FILTER (WHERE status = 'failed')     AS failed,
			COUNT(*) FILTER (WHERE status = 'cancelled')  AS cancelled
		FROM notifications WHERE batch_id = ?
	`, batchID).Scan(&s).Error
	if err != nil {
		return nil, fmt.Errorf("batch stats: %w", err)
	}

	var models []notificationModel
	if err := r.db.WithContext(ctx).
		Where("batch_id = ?", batchID).
		Order("created_at ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	notifications := make([]*domain.Notification, len(models))
	for i := range models {
		notifications[i] = toDomain(&models[i])
	}

	return &domain.BatchStatus{
		BatchID:       batchID,
		Total:         s.Total,
		Pending:       s.Pending,
		Processing:    s.Processing,
		Delivered:     s.Delivered,
		Failed:        s.Failed,
		Cancelled:     s.Cancelled,
		Notifications: notifications,
	}, nil
}

func (r *NotificationRepo) LogAttempt(ctx context.Context, a *domain.NotificationAttempt) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		Create(attemptToModel(a)).Error
}

func (r *NotificationRepo) GetAttempts(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationAttempt, error) {
	var models []notificationAttemptModel
	if err := r.db.WithContext(ctx).
		Where("notification_id = ?", notificationID).
		Order("attempt_number ASC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	attempts := make([]*domain.NotificationAttempt, len(models))
	for i := range models {
		attempts[i] = attemptToDomain(&models[i])
	}
	return attempts, nil
}
