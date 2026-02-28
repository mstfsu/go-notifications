package worker

import (
	"context"
	"encoding/json"
	"time"

	"github.com/hibiken/asynq"
	"github.com/mstfsu/go-case/internal/domain"
)

const TaskSendNotification = "notification:send"

const (
	QueueCritical = "critical"
	QueueDefault  = "default"
	QueueLow      = "low"
)

type NotificationPayload struct {
	NotificationID string `json:"notification_id"`
}

type Queue interface {
	Enqueue(ctx context.Context, notificationID string, priority domain.Priority, scheduledAt *time.Time) error
}

type asynqQueue struct {
	client     *asynq.Client
	maxRetries int
}

func NewQueue(client *asynq.Client, maxRetries int) Queue {
	return &asynqQueue{client: client, maxRetries: maxRetries}
}

func (q *asynqQueue) Enqueue(ctx context.Context, notificationID string, priority domain.Priority, scheduledAt *time.Time) error {
	payload, _ := json.Marshal(NotificationPayload{NotificationID: notificationID})
	opts := []asynq.Option{
		asynq.Queue(priorityToQueue(priority)),
		asynq.MaxRetry(q.maxRetries),
	}
	if scheduledAt != nil && scheduledAt.After(time.Now()) {
		opts = append(opts, asynq.ProcessAt(*scheduledAt))
	}
	_, err := q.client.EnqueueContext(ctx, asynq.NewTask(TaskSendNotification, payload), opts...)
	return err
}

func priorityToQueue(p domain.Priority) string {
	switch p {
	case domain.PriorityHigh:
		return QueueCritical
	case domain.PriorityLow:
		return QueueLow
	default:
		return QueueDefault
	}
}
