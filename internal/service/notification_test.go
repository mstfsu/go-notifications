package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/mstfsu/go-case/internal/domain"
	"github.com/mstfsu/go-case/internal/service"
)

// ---- Mock repository ----

type mockRepo struct {
	created   []*domain.Notification
	byID      map[string]*domain.Notification
	byIdemKey map[string]*domain.Notification
	cancelErr error
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		byID:      make(map[string]*domain.Notification),
		byIdemKey: make(map[string]*domain.Notification),
	}
}

func (m *mockRepo) Create(ctx context.Context, n *domain.Notification) error {
	m.created = append(m.created, n)
	m.byID[n.ID.String()] = n
	if n.IdempotencyKey != "" {
		m.byIdemKey[n.IdempotencyKey] = n
	}
	return nil
}
func (m *mockRepo) CreateBatch(ctx context.Context, notifications []*domain.Notification) error {
	for _, n := range notifications {
		m.created = append(m.created, n)
		m.byID[n.ID.String()] = n
	}
	return nil
}
func (m *mockRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Notification, error) {
	return m.byID[id.String()], nil
}
func (m *mockRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Notification, error) {
	return m.byIdemKey[key], nil
}
func (m *mockRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.Status, taskID string) error {
	return nil
}
func (m *mockRepo) UpdateDelivered(ctx context.Context, id uuid.UUID, providerMessageID string) error {
	return nil
}
func (m *mockRepo) UpdateFailed(ctx context.Context, id uuid.UUID, errorMessage string) error {
	return nil
}
func (m *mockRepo) IncrementRetry(ctx context.Context, id uuid.UUID) error { return nil }
func (m *mockRepo) Cancel(ctx context.Context, id uuid.UUID) error         { return m.cancelErr }
func (m *mockRepo) List(ctx context.Context, filter domain.ListFilter) (*domain.ListResult, error) {
	return &domain.ListResult{Notifications: m.created, Total: int64(len(m.created)), Page: filter.Page, Limit: filter.Limit}, nil
}
func (m *mockRepo) GetBatchStatus(ctx context.Context, batchID uuid.UUID) (*domain.BatchStatus, error) {
	return &domain.BatchStatus{BatchID: batchID, Total: int64(len(m.created))}, nil
}
func (m *mockRepo) LogAttempt(ctx context.Context, attempt *domain.NotificationAttempt) error {
	return nil
}
func (m *mockRepo) GetAttempts(ctx context.Context, notificationID uuid.UUID) ([]*domain.NotificationAttempt, error) {
	return nil, nil
}

// ---- Mock Queue ----

type mockQueue struct{ enqueueErr error }

func (m *mockQueue) Enqueue(_ context.Context, _ string, _ domain.Priority, _ *time.Time) error {
	return m.enqueueErr
}

func newSvc() *service.NotificationService {
	return service.NewNotificationService(newMockRepo(), &mockQueue{})
}

// ---- Tests ----

func TestCreate_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		req     *domain.CreateRequest
		wantErr error
	}{
		{
			name:    "invalid channel",
			req:     &domain.CreateRequest{Recipient: "+905551234567", Channel: "fax", Content: "hello", Priority: domain.PriorityNormal},
			wantErr: service.ErrInvalidChannel,
		},
		{
			name:    "invalid priority",
			req:     &domain.CreateRequest{Recipient: "+905551234567", Channel: domain.ChannelSMS, Content: "hello", Priority: "urgent"},
			wantErr: service.ErrInvalidPriority,
		},
		{
			name:    "sms content too long",
			req:     &domain.CreateRequest{Recipient: "+905551234567", Channel: domain.ChannelSMS, Content: string(make([]byte, 161)), Priority: domain.PriorityNormal},
			wantErr: service.ErrContentTooLong,
		},
		{
			name:    "invalid email",
			req:     &domain.CreateRequest{Recipient: "not-an-email", Channel: domain.ChannelEmail, Content: "hello", Priority: domain.PriorityNormal},
			wantErr: service.ErrInvalidRecipient,
		},
		{
			name:    "invalid phone",
			req:     &domain.CreateRequest{Recipient: "abc", Channel: domain.ChannelSMS, Content: "hello", Priority: domain.PriorityNormal},
			wantErr: service.ErrInvalidRecipient,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newSvc()
			_, _, err := svc.Create(context.Background(), tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestCreate_Success(t *testing.T) {
	svc := newSvc()
	n, isExisting, err := svc.Create(context.Background(), &domain.CreateRequest{
		Recipient:      "+905551234567",
		Channel:        domain.ChannelSMS,
		Content:        "Hello!",
		Priority:       domain.PriorityHigh,
		IdempotencyKey: "test-key-success",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isExisting {
		t.Error("should not be existing")
	}
	if n.ID == uuid.Nil {
		t.Error("ID should be set")
	}
	if n.Status != domain.StatusPending {
		t.Errorf("expected pending, got %s", n.Status)
	}
}

func TestCreate_DefaultPriority(t *testing.T) {
	svc := newSvc()
	req := &domain.CreateRequest{
		Recipient:      "abc@example.com",
		Channel:        domain.ChannelEmail,
		Content:        "hello",
		IdempotencyKey: "test-key-default-priority",
	}
	_, _, err := svc.Create(context.Background(), req)
	if errors.Is(err, service.ErrInvalidPriority) {
		t.Error("default priority should not cause ErrInvalidPriority")
	}
}

func TestCreate_Idempotency(t *testing.T) {
	svc := newSvc()
	req := &domain.CreateRequest{
		Recipient:      "+905551234567",
		Channel:        domain.ChannelSMS,
		Content:        "hello",
		Priority:       domain.PriorityNormal,
		IdempotencyKey: "my-unique-key",
	}
	n1, existing1, err := svc.Create(context.Background(), req)
	if err != nil || existing1 {
		t.Fatalf("first create: err=%v existing=%v", err, existing1)
	}
	n2, existing2, err := svc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if !existing2 {
		t.Error("second create should be idempotent")
	}
	if n1.ID != n2.ID {
		t.Error("idempotent response should return same notification")
	}
}

func TestCreateBatch_TooLarge(t *testing.T) {
	reqs := make([]*domain.CreateRequest, 1001)
	for i := range reqs {
		reqs[i] = &domain.CreateRequest{
			Recipient: "+905551234567",
			Channel:   domain.ChannelSMS,
			Content:   "hello",
			Priority:  domain.PriorityNormal,
		}
	}
	svc := newSvc()
	_, err := svc.CreateBatch(context.Background(), reqs)
	if !errors.Is(err, service.ErrBatchTooLarge) {
		t.Errorf("expected ErrBatchTooLarge, got %v", err)
	}
}

func TestCreateBatch_Success(t *testing.T) {
	svc := newSvc()
	reqs := []*domain.CreateRequest{
		{Recipient: "+905551234567", Channel: domain.ChannelSMS, Content: "msg1", Priority: domain.PriorityNormal, IdempotencyKey: "batch-key-1"},
		{Recipient: "+905551234568", Channel: domain.ChannelSMS, Content: "msg2", Priority: domain.PriorityHigh, IdempotencyKey: "batch-key-2"},
	}
	notifications, err := svc.CreateBatch(context.Background(), reqs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifications) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(notifications))
	}
	// All should share the same batch ID
	if *notifications[0].BatchID != *notifications[1].BatchID {
		t.Error("batch ID should be the same for all notifications in batch")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	svc := newSvc()
	_, err := svc.GetByID(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetByID_Found(t *testing.T) {
	svc := newSvc()
	created, _, _ := svc.Create(context.Background(), &domain.CreateRequest{
		Recipient:      "+905551234567",
		Channel:        domain.ChannelSMS,
		Content:        "hello",
		Priority:       domain.PriorityNormal,
		IdempotencyKey: "test-key-getbyid",
	})
	found, err := svc.GetByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != created.ID {
		t.Error("IDs should match")
	}
}

func TestCancel_NotFound(t *testing.T) {
	repo := newMockRepo()
	repo.cancelErr = errors.New("notification not found or not in pending state")
	svc := service.NewNotificationService(repo, &mockQueue{})
	err := svc.Cancel(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrCannotCancel) {
		t.Errorf("expected ErrCannotCancel, got %v", err)
	}
}

func TestList_DefaultPagination(t *testing.T) {
	svc := newSvc()
	result, err := svc.List(context.Background(), domain.ListFilter{Page: 0, Limit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 {
		t.Errorf("expected page 1, got %d", result.Page)
	}
	if result.Limit != 20 {
		t.Errorf("expected limit 20, got %d", result.Limit)
	}
}

func TestValidateRecipient_SMS(t *testing.T) {
	tests := []struct {
		phone   string
		wantErr bool
	}{
		{"+905551234567", false},
		{"+1234567890", false},
		{"abc", true},
		{"", true},
		{"123", true},
	}
	svc := newSvc()
	for _, tt := range tests {
		req := &domain.CreateRequest{
			Recipient: tt.phone,
			Channel:   domain.ChannelSMS,
			Content:   "hello",
			Priority:  domain.PriorityNormal,
		}
		_, _, err := svc.Create(context.Background(), req)
		hasErr := errors.Is(err, service.ErrInvalidRecipient)
		if hasErr != tt.wantErr {
			t.Errorf("phone=%q: wantErr=%v got err=%v", tt.phone, tt.wantErr, err)
		}
	}
}

func TestScheduledAt_FutureAllowed(t *testing.T) {
	future := time.Now().Add(1 * time.Hour)
	svc := newSvc()
	_, _, err := svc.Create(context.Background(), &domain.CreateRequest{
		Recipient:      "+905551234567",
		Channel:        domain.ChannelSMS,
		Content:        "hello",
		Priority:       domain.PriorityNormal,
		ScheduledAt:    &future,
		IdempotencyKey: "test-key-scheduled",
	})
	if errors.Is(err, service.ErrInvalidChannel) ||
		errors.Is(err, service.ErrInvalidPriority) ||
		errors.Is(err, service.ErrInvalidRecipient) ||
		errors.Is(err, service.ErrContentTooLong) {
		t.Errorf("unexpected validation error: %v", err)
	}
}
