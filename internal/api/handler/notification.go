package handler

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/mstfsu/go-case/internal/domain"
	"github.com/mstfsu/go-case/internal/service"
)

type NotificationHandler struct {
	svc    *service.NotificationService
	logger zerolog.Logger
}

func NewNotificationHandler(svc *service.NotificationService, logger zerolog.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, logger: logger}
}

func (h *NotificationHandler) ctxWithCorr(c *fiber.Ctx) context.Context {
	corrID, _ := c.Locals("correlation_id").(string)
	log := h.logger.With().Str("correlation_id", corrID).Logger()
	return log.WithContext(c.Context())
}

// @Summary     Create a notification
// @Tags        notifications
// @Accept      json
// @Produce     json
// @Param       body  body      createRequest       true  "Notification payload"
// @Success     201   {object}  domain.Notification
// @Success     200   {object}  domain.Notification
// @Failure     400   {object}  errorResponse
// @Failure     500   {object}  errorResponse
// @Router      /api/v1/notifications [post]
func (h *NotificationHandler) Create(c *fiber.Ctx) error {
	var req createRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	domainReq := &domain.CreateRequest{
		Recipient:      req.Recipient,
		Channel:        domain.Channel(req.Channel),
		Content:        req.Content,
		Priority:       domain.Priority(req.Priority),
		IdempotencyKey: req.IdempotencyKey,
		ScheduledAt:    req.ScheduledAt,
	}
	if domainReq.Priority == "" {
		domainReq.Priority = domain.PriorityNormal
	}

	n, isExisting, err := h.svc.Create(h.ctxWithCorr(c), domainReq)
	if err != nil {
		return handleServiceError(c, err)
	}
	if isExisting {
		return c.Status(fiber.StatusOK).JSON(n)
	}
	return c.Status(fiber.StatusCreated).JSON(n)
}

// @Summary     Create notifications in batch
// @Tags        notifications
// @Accept      json
// @Produce     json
// @Param       body  body      batchCreateRequest  true  "Batch payload"
// @Success     201   {object}  batchCreateResponse
// @Failure     400   {object}  errorResponse
// @Failure     500   {object}  errorResponse
// @Router      /api/v1/notifications/batch [post]
func (h *NotificationHandler) CreateBatch(c *fiber.Ctx) error {
	var req batchCreateRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}
	if len(req.Notifications) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "notifications list is empty")
	}

	domainReqs := make([]*domain.CreateRequest, 0, len(req.Notifications))
	for _, item := range req.Notifications {
		p := domain.Priority(item.Priority)
		if p == "" {
			p = domain.PriorityNormal
		}
		domainReqs = append(domainReqs, &domain.CreateRequest{
			Recipient:      item.Recipient,
			Channel:        domain.Channel(item.Channel),
			Content:        item.Content,
			Priority:       p,
			IdempotencyKey: item.IdempotencyKey,
			ScheduledAt:    item.ScheduledAt,
		})
	}

	notifications, err := h.svc.CreateBatch(h.ctxWithCorr(c), domainReqs)
	if err != nil {
		return handleServiceError(c, err)
	}

	batchID := ""
	if len(notifications) > 0 && notifications[0].BatchID != nil {
		batchID = notifications[0].BatchID.String()
	}
	return c.Status(fiber.StatusCreated).JSON(batchCreateResponse{
		BatchID: batchID,
		Count:   len(notifications),
	})
}

// @Summary     Get notification by ID
// @Tags        notifications
// @Produce     json
// @Param       id   path      string  true  "Notification ID (UUID)"
// @Success     200  {object}  domain.Notification
// @Failure     400  {object}  errorResponse
// @Failure     404  {object}  errorResponse
// @Router      /api/v1/notifications/{id} [get]
func (h *NotificationHandler) GetByID(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid notification ID")
	}
	n, err := h.svc.GetByID(h.ctxWithCorr(c), id)
	if err != nil {
		return handleServiceError(c, err)
	}
	return c.JSON(n)
}

// @Summary     Get batch status
// @Tags        notifications
// @Produce     json
// @Param       batchId  path      string  true  "Batch ID (UUID)"
// @Success     200      {object}  domain.BatchStatus
// @Failure     400      {object}  errorResponse
// @Failure     404      {object}  errorResponse
// @Router      /api/v1/notifications/batch/{batchId} [get]
func (h *NotificationHandler) GetBatchStatus(c *fiber.Ctx) error {
	batchID, err := uuid.Parse(c.Params("batchId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid batch ID")
	}
	bs, err := h.svc.GetBatchStatus(h.ctxWithCorr(c), batchID)
	if err != nil {
		return handleServiceError(c, err)
	}
	return c.JSON(bs)
}

// @Summary     Cancel a pending notification
// @Tags        notifications
// @Produce     json
// @Param       id   path      string  true  "Notification ID (UUID)"
// @Success     200  {object}  map[string]string
// @Failure     400  {object}  errorResponse
// @Failure     404  {object}  errorResponse
// @Failure     409  {object}  errorResponse
// @Router      /api/v1/notifications/{id} [delete]
func (h *NotificationHandler) Cancel(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid notification ID")
	}
	if err := h.svc.Cancel(h.ctxWithCorr(c), id); err != nil {
		return handleServiceError(c, err)
	}
	return c.JSON(fiber.Map{"message": "notification cancelled"})
}

// @Summary     List notifications
// @Tags        notifications
// @Produce     json
// @Param       status    query     string  false  "Filter by status (pending|processing|delivered|failed|cancelled)"
// @Param       channel   query     string  false  "Filter by channel (sms|email|push)"
// @Param       date_from query     string  false  "From date (RFC3339)"
// @Param       date_to   query     string  false  "To date (RFC3339)"
// @Param       page      query     int     false  "Page number (default: 1)"
// @Param       limit     query     int     false  "Page size (default: 20, max: 100)"
// @Success     200       {object}  domain.ListResult
// @Failure     400       {object}  errorResponse
// @Router      /api/v1/notifications [get]
func (h *NotificationHandler) List(c *fiber.Ctx) error {
	filter := domain.ListFilter{
		Status:  c.Query("status"),
		Channel: c.Query("channel"),
		Page:    c.QueryInt("page", 1),
		Limit:   c.QueryInt("limit", 20),
	}
	if df := c.Query("date_from"); df != "" {
		t, err := time.Parse(time.RFC3339, df)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid date_from format, use RFC3339")
		}
		filter.DateFrom = &t
	}
	if dt := c.Query("date_to"); dt != "" {
		t, err := time.Parse(time.RFC3339, dt)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid date_to format, use RFC3339")
		}
		filter.DateTo = &t
	}
	result, err := h.svc.List(h.ctxWithCorr(c), filter)
	if err != nil {
		return handleServiceError(c, err)
	}
	return c.JSON(result)
}

type createRequest struct {
	Recipient      string     `json:"recipient"`
	Channel        string     `json:"channel"`
	Content        string     `json:"content"`
	Priority       string     `json:"priority"`
	IdempotencyKey string     `json:"idempotency_key"`
	ScheduledAt    *time.Time `json:"scheduled_at,omitempty"`
}

type batchCreateRequest struct {
	Notifications []createRequest `json:"notifications"`
}

type batchCreateResponse struct {
	BatchID string `json:"batch_id"`
	Count   int    `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func handleServiceError(c *fiber.Ctx, err error) error {
	switch err {
	case service.ErrNotFound:
		return fiber.NewError(fiber.StatusNotFound, err.Error())
	case service.ErrCannotCancel:
		return fiber.NewError(fiber.StatusConflict, err.Error())
	case service.ErrBatchTooLarge:
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	case service.ErrInvalidChannel, service.ErrInvalidPriority,
		service.ErrContentTooLong, service.ErrInvalidRecipient:
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	default:
		zerolog.Ctx(c.Context()).Error().Err(err).Msg("unhandled service error")
		return fiber.NewError(fiber.StatusInternalServerError, "internal server error")
	}
}
