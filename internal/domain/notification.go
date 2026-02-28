package domain

import (
	"time"

	"github.com/google/uuid"
)

type Channel string
type Priority string
type Status string

const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"

	PriorityHigh   Priority = "high"
	PriorityNormal Priority = "normal"
	PriorityLow    Priority = "low"

	StatusPending    Status = "pending"
	StatusProcessing Status = "processing"
	StatusDelivered  Status = "delivered"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
)

func (c Channel) IsValid() bool {
	switch c {
	case ChannelSMS, ChannelEmail, ChannelPush:
		return true
	}
	return false
}

func (p Priority) IsValid() bool {
	switch p {
	case PriorityHigh, PriorityNormal, PriorityLow:
		return true
	}
	return false
}

type Notification struct {
	ID                uuid.UUID  `json:"id"`
	BatchID           *uuid.UUID `json:"batch_id,omitempty"`
	Recipient         string     `json:"recipient"`
	Channel           Channel    `json:"channel"`
	Content           string     `json:"content"`
	Priority          Priority   `json:"priority"`
	Status            Status     `json:"status"`
	IdempotencyKey    string     `json:"idempotency_key,omitempty"`
	TaskID            string     `json:"task_id,omitempty"`
	ProviderMessageID string     `json:"provider_message_id,omitempty"`
	ScheduledAt       *time.Time `json:"scheduled_at,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	RetryCount        int        `json:"retry_count"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type NotificationAttempt struct {
	ID               uuid.UUID      `json:"id"`
	NotificationID   uuid.UUID      `json:"notification_id"`
	AttemptNumber    int            `json:"attempt_number"`
	Status           string         `json:"status"`
	ErrorMessage     string         `json:"error_message,omitempty"`
	ProviderResponse map[string]any `json:"provider_response,omitempty"`
	AttemptedAt      time.Time      `json:"attempted_at"`
}

type CreateRequest struct {
	Recipient      string     `json:"recipient"`
	Channel        Channel    `json:"channel"`
	Content        string     `json:"content"`
	Priority       Priority   `json:"priority"`
	IdempotencyKey string     `json:"idempotency_key,omitempty"`
	ScheduledAt    *time.Time `json:"scheduled_at,omitempty"`
}

type ListFilter struct {
	Status   string
	Channel  string
	DateFrom *time.Time
	DateTo   *time.Time
	Page     int
	Limit    int
}

type ListResult struct {
	Notifications []*Notification `json:"notifications"`
	Total         int64           `json:"total"`
	Page          int             `json:"page"`
	Limit         int             `json:"limit"`
}

type BatchStatus struct {
	BatchID       uuid.UUID       `json:"batch_id"`
	Total         int64           `json:"total"`
	Pending       int64           `json:"pending"`
	Processing    int64           `json:"processing"`
	Delivered     int64           `json:"delivered"`
	Failed        int64           `json:"failed"`
	Cancelled     int64           `json:"cancelled"`
	Notifications []*Notification `json:"notifications"`
}
