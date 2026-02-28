package postgres

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/mstfsu/go-case/internal/domain"
)

type notificationModel struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	BatchID           *uuid.UUID `gorm:"type:uuid;index:idx_notifications_batch_id"`
	Recipient         string     `gorm:"type:varchar(255);not null"`
	Channel           string     `gorm:"type:varchar(50);not null;index:idx_notifications_channel"`
	Content           string     `gorm:"type:text;not null"`
	Priority          string     `gorm:"type:varchar(20);not null;default:normal"`
	Status            string     `gorm:"type:varchar(20);not null;default:pending;index:idx_notifications_status"`
	IdempotencyKey    string     `gorm:"type:varchar(255);uniqueIndex"`
	TaskID            string     `gorm:"type:varchar(255)"`
	ProviderMessageID string     `gorm:"type:varchar(255)"`
	ScheduledAt       *time.Time `gorm:"index:idx_notifications_scheduled"`
	ErrorMessage      string     `gorm:"type:text"`
	RetryCount        int        `gorm:"not null;default:0"`
	CreatedAt         time.Time  `gorm:"index:idx_notifications_created_at"`
	UpdatedAt         time.Time
}

func (notificationModel) TableName() string { return "notifications" }

type notificationAttemptModel struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	NotificationID   uuid.UUID `gorm:"type:uuid;not null;index:idx_attempts_notification_id"`
	AttemptNumber    int       `gorm:"not null"`
	Status           string    `gorm:"type:varchar(20);not null"`
	ErrorMessage     string    `gorm:"type:text"`
	ProviderResponse []byte    `gorm:"type:jsonb"`
	AttemptedAt      time.Time `gorm:"not null"`
}

func (notificationAttemptModel) TableName() string { return "notification_attempts" }

func toModel(n *domain.Notification) *notificationModel {
	return &notificationModel{
		ID:                n.ID,
		BatchID:           n.BatchID,
		Recipient:         n.Recipient,
		Channel:           string(n.Channel),
		Content:           n.Content,
		Priority:          string(n.Priority),
		Status:            string(n.Status),
		IdempotencyKey:    n.IdempotencyKey,
		TaskID:            n.TaskID,
		ProviderMessageID: n.ProviderMessageID,
		ScheduledAt:       n.ScheduledAt,
		ErrorMessage:      n.ErrorMessage,
		RetryCount:        n.RetryCount,
		CreatedAt:         n.CreatedAt,
		UpdatedAt:         n.UpdatedAt,
	}
}

func toDomain(m *notificationModel) *domain.Notification {
	return &domain.Notification{
		ID:                m.ID,
		BatchID:           m.BatchID,
		Recipient:         m.Recipient,
		Channel:           domain.Channel(m.Channel),
		Content:           m.Content,
		Priority:          domain.Priority(m.Priority),
		Status:            domain.Status(m.Status),
		IdempotencyKey:    m.IdempotencyKey,
		TaskID:            m.TaskID,
		ProviderMessageID: m.ProviderMessageID,
		ScheduledAt:       m.ScheduledAt,
		ErrorMessage:      m.ErrorMessage,
		RetryCount:        m.RetryCount,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

func attemptToModel(a *domain.NotificationAttempt) *notificationAttemptModel {
	providerJSON, _ := json.Marshal(a.ProviderResponse)
	return &notificationAttemptModel{
		ID:               a.ID,
		NotificationID:   a.NotificationID,
		AttemptNumber:    a.AttemptNumber,
		Status:           a.Status,
		ErrorMessage:     a.ErrorMessage,
		ProviderResponse: providerJSON,
		AttemptedAt:      a.AttemptedAt,
	}
}

func attemptToDomain(m *notificationAttemptModel) *domain.NotificationAttempt {
	a := &domain.NotificationAttempt{
		ID:             m.ID,
		NotificationID: m.NotificationID,
		AttemptNumber:  m.AttemptNumber,
		Status:         m.Status,
		ErrorMessage:   m.ErrorMessage,
		AttemptedAt:    m.AttemptedAt,
	}
	if len(m.ProviderResponse) > 0 {
		_ = json.Unmarshal(m.ProviderResponse, &a.ProviderResponse)
	}
	return a
}
