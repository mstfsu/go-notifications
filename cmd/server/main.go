// @title           Notification Service API
// @version         1.0
// @description     Event-driven notification system — SMS, Email, Push delivery via async queue workers.
// @host            localhost:8080
// @BasePath        /

package main

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/hibiken/asynq"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/mstfsu/go-case/internal/api"
	"github.com/mstfsu/go-case/internal/api/handler"
	"github.com/mstfsu/go-case/internal/config"
	"github.com/mstfsu/go-case/internal/provider"
	pgRepo "github.com/mstfsu/go-case/internal/repository/postgres"
	"github.com/mstfsu/go-case/internal/service"
	"github.com/mstfsu/go-case/internal/worker"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	_ "github.com/mstfsu/go-case/docs"
)

func main() {
	_ = godotenv.Load()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Caller().Logger()

	cfg := config.Load()

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatal().Err(err).Msg("migrations failed")
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal().Err(err).Msg("connect to postgres")
	}
	sqlDB, _ := db.DB()
	defer func() { _ = sqlDB.Close() }()
	log.Info().Msg("postgres connected")

	redisOpts, err := parseRedisURL(cfg.RedisURL)
	if err != nil {
		log.Fatal().Err(err).Msg("parse redis url")
	}
	redisClient := redis.NewClient(redisOpts)
	defer func() { _ = redisClient.Close() }()
	log.Info().Msg("redis connected")

	asynqRedisOpt := asynq.RedisClientOpt{
		Addr:     redisOpts.Addr,
		Password: redisOpts.Password,
		DB:       redisOpts.DB,
	}

	notifRepo := pgRepo.NewNotificationRepo(db)

	asynqClient := asynq.NewClient(asynqRedisOpt)
	defer func() { _ = asynqClient.Close() }()

	webhookProvider := provider.NewWebhookProvider(cfg.WebhookSiteURL)

	queue := worker.NewQueue(asynqClient, cfg.MaxRetries)
	notifService := service.NewNotificationService(notifRepo, queue)

	processor := worker.NewProcessor(notifRepo, webhookProvider, log.Logger)
	asynqServer := worker.NewServer(asynqRedisOpt, cfg.WorkerConcurrency, log.Logger)
	mux := asynq.NewServeMux()
	worker.Register(mux, processor)

	inspector := asynq.NewInspector(asynqRedisOpt)

	notifHandler := handler.NewNotificationHandler(notifService, log.Logger)
	metricsHandler := handler.NewMetricsHandler(inspector, db, redisClient)

	app := api.SetupRouter(notifHandler, metricsHandler)

	go func() {
		if err := asynqServer.Run(mux); err != nil {
			log.Fatal().Err(err).Msg("asynq server error")
		}
	}()
	log.Info().Int("concurrency", cfg.WorkerConcurrency).Msg("worker started")

	go func() {
		addr := fmt.Sprintf(":%s", cfg.ServerPort)
		log.Info().Str("addr", addr).Msg("HTTP server starting")
		if err := app.Listen(addr); err != nil {
			log.Fatal().Err(err).Msg("fiber server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down...")

	asynqServer.Shutdown()
	if err := app.Shutdown(); err != nil {
		log.Error().Err(err).Msg("fiber shutdown error")
	}
	log.Info().Msg("shutdown complete")
}

func runMigrations(dbURL string) error {
	m, err := migrate.New("file://migrations", dbURL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Info().Msg("migrations applied")
	return nil
}

func parseRedisURL(redisURL string) (*redis.Options, error) {
	u, err := url.Parse(redisURL)
	if err != nil {
		return nil, err
	}
	opts := &redis.Options{Addr: u.Host}
	if u.User != nil {
		opts.Password, _ = u.User.Password()
	}
	return opts, nil
}
