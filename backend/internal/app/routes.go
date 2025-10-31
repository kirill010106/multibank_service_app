package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// setupRoutes configures all application routes
func (a *App) setupRoutes() {
	const op = "app.setupRoutes"

	// Root endpoint
	a.server.Get("/", a.handleRoot)

	// Health check endpoint
	a.server.Get("/health", a.handleHealth)

	// Auth routes with rate limiting
	a.setupAuthRoutes()

	// Secure routes

	a.setupProtectedRoutes()

	a.log.Info().Msg("routes configured")
}

// handleRoot handles GET /
func (a *App) handleRoot(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"message": "VTB Multibank API",
		"version": "1.0.0",
		"env":     a.config.Env,
	})
}

// handleHealth handles GET /health
func (a *App) handleHealth(c *fiber.Ctx) error {
	if err := a.db.Ping(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "unhealthy",
			"error":  "database unreachable",
		})
	}
	return c.JSON(fiber.Map{
		"status": "healthy",
	})
}

// setupAuthRoutes configures authentication routes with rate limiting
func (a *App) setupAuthRoutes() {
	const op = "app.setupAuthRoutes"

	authGroup := a.server.Group("/api/v1/auth")

	// Rate limiter: 5 requests per minute per IP
	authGroup.Use(limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c *fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c *fiber.Ctx) error {
			a.log.Warn().
				Str("ip", c.IP()).
				Str("op", op).
				Msg("rate limit exceeded")
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests, try again later",
			})
		},
	}))

	// Auth endpoints
	authGroup.Post("/register", a.authHandler.Register)
	authGroup.Post("/login", a.authHandler.Login)

	a.log.Info().Msg("routes configured")

}

func (a *App) setupProtectedRoutes() {
	const op = "app.setupProtectedRoutes"

	protectedGroup := a.server.Group("/api/v1")
	protectedGroup.Use(a.GetJWTMiddleware())

	protectedGroup.Get("/me", a.handleGetCurrentUser)
	protectedGroup.Get("/dashboard", a.handleGetDashboard)

	a.log.Info().Msg("protected routes configured")
}

// handleGetCurrentUser handles GET /api/v1/me
// return current user info from JWT token

func (a *App) handleGetCurrentUser(c *fiber.Ctx) error {
	const op = "app.handleGetCurrentUser"
	userID := c.Locals("user_id").(int)
	email := c.Locals("email").(string)

	a.log.Debug().
		Str("op", op).
		Int("user_id", userID).
		Str("email", email).
		Msg("fetched current user info")

	return c.JSON(fiber.Map{
		"user_id": userID,
		"email":   email,
	})
}

func (a *App) handleGetDashboard(c *fiber.Ctx) error {
	const op = "app.handleGetDashboard"

	userID := c.Locals("user_id").(int)

	a.log.Debug().
		Str("op", op).
		Int("user_id", userID).
		Msg("fetched dashboard info")

	// TODO: Implement bank service integration

	return c.JSON(fiber.Map{
		"user_id": userID,
		"message": "Dashboard data will be here soon",
		"banks":   []string{},
	})
}
