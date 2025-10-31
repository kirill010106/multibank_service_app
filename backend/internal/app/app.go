package app

import (
	"database/sql"
	"time"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/kirill010106/multibank_service_app/backend/internal/config"
	"github.com/kirill010106/multibank_service_app/backend/internal/handlers"
	"github.com/kirill010106/multibank_service_app/backend/internal/middleware"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/auth"
	_ "github.com/lib/pq"
	"github.com/rs/zerolog"
)

// App represents the application with all dependencies
type App struct {
	config      *config.Config
	server      *fiber.App
	db          *sql.DB
	log         *zerolog.Logger
	authService *auth.Service
	authHandler *handlers.AuthHandler
	jwtManager  *auth.JWTManager
}

// New creates a new App instance with all dependencies initialized
func New(cfg *config.Config, log *zerolog.Logger) (*App, error) {
	const op = "app.New"

	app := &App{
		config: cfg,
		log:    log,
	}

	// Initialize database
	if err := app.initDB(); err != nil {
		return nil, err
	}

	// Run migrations
	if err := app.runMigrations(); err != nil {
		return nil, err
	}

	// Initialize services
	app.initServices()

	// Initialize handlers
	app.initHandlers()

	// Initialize Fiber server
	app.initServer()

	// Setup routes
	app.setupRoutes()

	return app, nil
}

// initDB initializes database connection with proper pool settings
func (a *App) initDB() error {
	const op = "app.initDB"

	db, err := sql.Open("postgres", a.config.StorageURL)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to open db connection")
		return err
	}

	// Connection pool settings optimized for Clever Cloud free tier (5 connection limit)
	db.SetMaxOpenConns(3)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(3 * time.Minute)
	db.SetConnMaxIdleTime(30 * time.Second)

	// Verify connection with retries
	a.log.Info().Msg("connecting to database...")
	for i := 0; i < 3; i++ {
		if err := db.Ping(); err != nil {
			a.log.Warn().
				Err(err).
				Msgf("database ping attempt %d failed", i+1)
			time.Sleep(2 * time.Second)
			continue
		}
		break
	}

	if err := db.Ping(); err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to ping database after 3 attempts")
		return err
	}

	a.db = db
	a.log.Info().Msg("database connection established")
	return nil
}

// runMigrations applies database migrations
func (a *App) runMigrations() error {
	const op = "app.runMigrations"

	driver, err := postgres.WithInstance(a.db, &postgres.Config{})
	if err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to create migrate driver")
		return err
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		"postgres",
		driver,
	)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to create migrate instance")
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to run migrations")
		return err
	}

	a.log.Info().Msg("migrations applied successfully")
	return nil
}

// initServices initializes all business logic services
func (a *App) initServices() {
	// JWT Manager
	a.jwtManager = auth.NewJWTManager(
		a.config.JWTSecret,
		24*time.Hour,
	)

	// Auth Service
	a.authService = auth.NewService(a.db, a.jwtManager)

	a.log.Info().Msg("services initialized")
}

// initHandlers initializes all HTTP handlers
func (a *App) initHandlers() {
	a.authHandler = handlers.NewAuthHandler(a.authService, *a.log)

	a.log.Info().Msg("handlers initialized")
}

// initServer initializes Fiber server with global middleware
func (a *App) initServer() {
	a.server = fiber.New(fiber.Config{
		AppName: "VTB Multibank API",
	})

	// Global middleware
	a.server.Use(fiberzerolog.New(fiberzerolog.Config{
		Logger: a.log,
	}))
	a.server.Use(recover.New())

	a.log.Info().Msg("server initialized")
}

// GetJWTMiddleware return configured JWT authentication middleware

func (a *App) GetJWTMiddleware() fiber.Handler {
	return middleware.JWTAuth(a.jwtManager, *a.log)
}

// Run starts the HTTP server
func (a *App) Run() error {
	const op = "app.Run"

	address := a.config.HTTPServer.Address
	a.log.Info().
		Str("address", address).
		Str("env", a.config.Env).
		Msg("starting server...")

	if err := a.server.Listen(address); err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to start server")
		return err
	}

	return nil
}

// Close gracefully shuts down the application
func (a *App) Close() error {
	a.log.Info().Msg("shutting down application...")

	if a.db != nil {
		if err := a.db.Close(); err != nil {
			a.log.Error().Err(err).Msg("error closing database")
			return err
		}
	}

	if a.server != nil {
		if err := a.server.Shutdown(); err != nil {
			a.log.Error().Err(err).Msg("error shutting down server")
			return err
		}
	}

	a.log.Info().Msg("application stopped")
	return nil
}
