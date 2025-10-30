package main

import (
	"database/sql"
	"time"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/kirill010106/multibank_service_app/backend/internal/config"
	"github.com/kirill010106/multibank_service_app/backend/internal/handlers"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/auth"
	"github.com/kirill010106/multibank_service_app/backend/pkg/logger"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

func main() {
	const op = "cmd.main"
	// Инициализация конфигурации
	config.Init()
	cfg := config.MustLoad()

	// Инициализация логгера
	logConfig := config.NewLogConfig()
	log := logger.NewLogger(logConfig)
	zerolog.SetGlobalLevel(zerolog.Level(logConfig.Level))

	// Подключение к БД
	db, err := sql.Open("postgres", cfg.StorageURL)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("op", op).
			Msg("failed to connect to db")
	}
	defer db.Close()

	db.SetMaxOpenConns(3)                   // макс открытых соединений
	db.SetMaxIdleConns(1)                   // макс idle соединений
	db.SetConnMaxLifetime(3 * time.Minute)  // соединение живет макс 3 минут
	db.SetConnMaxIdleTime(30 * time.Second) // idle соединение живет макс 30 секунд

	log.Info().Msg("connecting to db...")
	for i := 0; i < 3; i++ {
		if err := db.Ping(); err != nil {
			log.Warn().
				Err(err).
				Msgf("ping db attempt %d failed", i+1)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	if err := db.Ping(); err != nil {
		log.Fatal().
			Err(err).
			Str("op", op).
			Msg("failed to ping db after 3 attempts")
	}
	log.Info().Msg("db connection established")

	// Auto migrations start

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatal().
			Err(err).
			Str("op", op).
			Msg("failed to create migrate driver")
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres", driver)
	if err != nil {
		log.Fatal().
			Err(err).
			Str("op", op).
			Msg("failed to create migrate instance")
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatal().
			Err(err).
			Str("op", op).
			Msg("failed to run migrate up")
	}
	log.Info().Msg("migrations applied successfully")

	// Auto migrations end

	// JWT Manager

	jwtManager := auth.NewJWTManager(
		cfg.JWTSecret,
		24*time.Hour,
	)

	// Auth Service

	authService := auth.NewService(db, jwtManager)

	// Auth Handler

	authHandler := handlers.NewAuthHandler(authService, *log)

	// Логирование запуска
	logger.LogServerStart(log, cfg)

	// Создание Fiber приложения
	app := fiber.New(fiber.Config{
		AppName: "VTB Multibank API",
	})

	// Middleware
	app.Use(fiberzerolog.New(fiberzerolog.Config{
		Logger: log,
	}))
	app.Use(recover.New())

	// Базовые роуты
	app.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"message": "VTB Multibank API",
			"version": "1.0.0",
			"env":     cfg.Env,
		})
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		if err := db.Ping(); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "unhealthy",
				"error":  "db unreachable",
			})
		}
		return c.JSON(fiber.Map{
			"status": "healthy",
		})
	})

	// AUTH

	authGroup := app.Group("/auth")

	authGroup.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			log.Warn().
				Str("ip", c.IP()).
				Str("op", op).
				Msg("rate limit exceeded")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, try again later",
			})
		},
	}))

	authGroup.Post("/register", authHandler.Register)
	authGroup.Post("/login", authHandler.Login)

	// Запуск сервера
	address := cfg.HTTPServer.Address
	logger.LogServerListening(log, address)

	if err := app.Listen(address); err != nil {
		log.Fatal().Err(err).Str("op", op).Msg("Failed to start server")
	}
}
