package app

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"

	_ "github.com/kirill010106/multibank_service_app/backend/docs" // ← Сгенерированная документация
	swagger "github.com/swaggo/fiber-swagger"
)

// setupRoutes configures all application routes
func (a *App) setupRoutes() {
	const op = "app.setupRoutes"

	// Root endpoint
	a.server.Get("/", a.handleRoot)

	// Health check endpoint
	a.server.Get("/health", a.handleHealth)

	// Swagger documentation
	a.server.Get("/swagger/*", swagger.WrapHandler)

	// Auth routes with rate limiting
	a.setupAuthRoutes()

	// Secure routes

	a.setupProtectedRoutes()

	a.setupBankRoutes()

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

// handleGetCurrentUser godoc
// @Summary      Get current user info
// @Description  Get authenticated user information from JWT token
// @Tags         user
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{} "User info"
// @Failure      401 {object} handlers.ErrorResponse "Unauthorized"
// @Router       /me [get]
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

// handleGetDashboard godoc
// @Summary      Get dashboard data
// @Description  Retrieve aggregated data from all connected banks
// @Tags         dashboard
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} models.DashboardResponse "Dashboard data"
// @Failure      401 {object} handlers.ErrorResponse "Unauthorized"
// @Failure      500 {object} handlers.ErrorResponse "Internal server error"
// @Router       /dashboard [get]
func (a *App) handleGetDashboard(c *fiber.Ctx) error {
	const op = "app.handleGetDashboard"

	userID := c.Locals("user_id").(int)

	a.log.Debug().
		Str("op", op).
		Int("user_id", userID).
		Msg("fetching dashboard data")

	// Fetch aggregated dashboard data from bank service
	dashboard, err := a.bankService.GetDashboard(c.Context(), userID)
	if err != nil {
		a.log.Error().
			Err(err).
			Str("op", op).
			Int("user_id", userID).
			Msg("failed to fetch dashboard")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to fetch dashboard data",
		})
	}

	a.log.Info().
		Str("op", op).
		Int("user_id", userID).
		Int("total_banks", dashboard.TotalBanks).
		Int("active_banks", dashboard.ActiveBanks).
		Int("total_accounts", dashboard.TotalAccounts).
		Msg("dashboard data fetched successfully")

	return c.JSON(dashboard)
}

// setupBankRoutes configures bank integration routes (JWT protected)
func (a *App) setupBankRoutes() {
	const op = "app.setupBankRoutes"

	// Bank routes group with JWT middleware
	bankGroup := a.server.Group("/api/v1/banks")
	bankGroup.Use(a.GetJWTMiddleware())

	// Bank endpoints
	bankGroup.Post("/connect", a.bankHandler.ConnectBank)                                       // POST /api/v1/banks/connect
	bankGroup.Get("/", a.bankHandler.GetConnections)                                            // GET /api/v1/banks
	bankGroup.Get("/:provider/accounts", a.bankHandler.GetAccounts)                             // GET /api/v1/banks/:provider/accounts
	bankGroup.Get("/:provider/accounts/:accountId/balances", a.bankHandler.GetBalances)         // GET /api/v1/banks/:provider/accounts/:accountId/balances
	bankGroup.Get("/:provider/accounts/:accountId/transactions", a.bankHandler.GetTransactions) // GET /api/v1/banks/:provider/accounts/:accountId/transactions
	bankGroup.Delete("/:provider", a.bankHandler.DisconnectBank)                                // DELETE /api/v1/banks/:provider

	a.log.Info().Msg("bank routes configured")
}
