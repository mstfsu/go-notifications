package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type MetricsHandler struct {
	inspector *asynq.Inspector
	db        *gorm.DB
	redis     *redis.Client
	startTime time.Time
}

func NewMetricsHandler(inspector *asynq.Inspector, db *gorm.DB, redisClient *redis.Client) *MetricsHandler {
	return &MetricsHandler{
		inspector: inspector,
		db:        db,
		redis:     redisClient,
		startTime: time.Now(),
	}
}

type queueStats struct {
	Size      int `json:"size"`
	Active    int `json:"active"`
	Scheduled int `json:"scheduled"`
	Retry     int `json:"retry"`
	Archived  int `json:"archived"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
}

type metricsResponse struct {
	Queues        map[string]*queueStats `json:"queues"`
	UptimeSeconds int64                  `json:"uptime_seconds"`
}

// @Summary     Queue metrics
// @Tags        observability
// @Produce     json
// @Success     200  {object}  metricsResponse
// @Router      /api/v1/metrics [get]
func (h *MetricsHandler) GetMetrics(c *fiber.Ctx) error {
	queues := []string{"critical", "default", "low"}
	stats := make(map[string]*queueStats, len(queues))

	for _, q := range queues {
		info, err := h.inspector.GetQueueInfo(q)
		if err != nil {
			stats[q] = &queueStats{}
			continue
		}
		stats[q] = &queueStats{
			Size:      info.Size,
			Active:    info.Active,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
			Failed:    info.Failed,
		}
	}

	return c.JSON(metricsResponse{
		Queues:        stats,
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
	})
}

// @Summary     Health check
// @Tags        observability
// @Produce     json
// @Success     200  {object}  map[string]string
// @Failure     503  {object}  map[string]string
// @Router      /health [get]
func (h *MetricsHandler) HealthCheck(c *fiber.Ctx) error {
	status := fiber.Map{
		"status":   "ok",
		"postgres": "ok",
		"redis":    "ok",
	}
	code := fiber.StatusOK

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.PingContext(c.Context()) != nil {
		status["postgres"] = "unavailable"
		status["status"] = "degraded"
		code = fiber.StatusServiceUnavailable
	}

	if err := h.redis.Ping(c.Context()).Err(); err != nil {
		status["redis"] = "unavailable"
		status["status"] = "degraded"
		code = fiber.StatusServiceUnavailable
	}

	return c.Status(code).JSON(status)
}
