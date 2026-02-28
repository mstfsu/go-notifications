package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	fiberSwagger "github.com/swaggo/fiber-swagger"
	"github.com/mstfsu/go-case/internal/api/handler"
	"github.com/mstfsu/go-case/internal/api/middleware"
)

func SetupRouter(
	notifHandler *handler.NotificationHandler,
	metricsHandler *handler.MetricsHandler,
) *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			msg := "internal server error"
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
				msg = e.Message
			}
			return c.Status(code).JSON(fiber.Map{"error": msg})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "${time} | ${status} | ${latency} | ${ip} | ${method} ${path} | corr=${locals:correlation_id}\n",
	}))
	app.Use(middleware.CorrelationID())

	app.Get("/health", metricsHandler.HealthCheck)
	app.Get("/swagger/*", fiberSwagger.WrapHandler)

	v1 := app.Group("/api/v1")

	notifications := v1.Group("/notifications")
	notifications.Post("/", notifHandler.Create)
	notifications.Post("/batch", notifHandler.CreateBatch)
	notifications.Get("/batch/:batchId", notifHandler.GetBatchStatus)
	notifications.Get("/:id", notifHandler.GetByID)
	notifications.Delete("/:id", notifHandler.Cancel)
	notifications.Get("/", notifHandler.List)

	v1.Get("/metrics", metricsHandler.GetMetrics)

	return app
}
