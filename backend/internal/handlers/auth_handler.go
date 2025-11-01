package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/auth"
	"github.com/rs/zerolog"
)

// RegisterRequest represents registration request body
type RegisterRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"password123"`
}

// RegisterResponse represents registration response
type RegisterResponse struct {
	Token string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserPayload `json:"user"`
}

// UserPayload represents user info in response
type UserPayload struct {
	ID       int    `json:"id" example:"1"`
	Email    string `json:"email" example:"user@example.com"`
	ClientID string `json:"client_id" example:"team200"`
}

// LoginRequest represents login request body
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"user@example.com"`
	Password string `json:"password" validate:"required" example:"password123"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Token string      `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	User  UserPayload `json:"user"`
}

type AuthHandler struct {
	authService *auth.Service
	log         zerolog.Logger
	validate    *validator.Validate
}

// formatValidationErrors преобразует ошибки validator в читаемый формат
func (h *AuthHandler) formatValidationErrors(err error) map[string]string {
	validationErrors := make(map[string]string)
	for _, e := range err.(validator.ValidationErrors) {
		field := e.Field()
		switch e.Tag() {
		case "required":
			validationErrors[field] = field + " is required"
		case "email":
			validationErrors[field] = field + " must be a valid email"
		case "min":
			validationErrors[field] = field + " must be at least " + e.Param() + " characters"
		default:
			validationErrors[field] = field + " is invalid"
		}
	}
	return validationErrors
}

func NewAuthHandler(authService *auth.Service, log zerolog.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		log:         log,
		validate:    validator.New(),
	}
}

// Register godoc
// @Summary      Register new user
// @Description  Create a new user account with email and password
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Registration credentials"
// @Success      201 {object} RegisterResponse "User created successfully"
// @Failure      400 {object} ErrorResponse "Invalid request body or validation error"
// @Failure      409 {object} ErrorResponse "User already exists"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /auth/register [post]
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	const op = "handlers.AuthHandler.Register"

	var req models.RegisterRequest

	if err := c.BodyParser(&req); err != nil {
		h.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to parse request body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if err := h.validate.Struct(&req); err != nil {
		h.log.Warn().
			Err(err).
			Str("op", op).
			Msg("validation failed for registration request")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "validation failed",
			"fields": h.formatValidationErrors(err),
		})
	}

	response, err := h.authService.Register(&req)

	if err != nil {
		h.log.Error().
			Err(err).
			Str("op", op).
			Str("email", req.Email).
			Msg("registration failed")

		if errors.Is(err, models.ErrEmailAlreadyExists) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"error": "email already registered",
			})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "registration failed",
		})
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", response.User.ID).
		Str("email", response.User.Email).
		Msg("user registered successfully")

	return c.Status(fiber.StatusCreated).JSON(response)

}

// Login godoc
// @Summary      User login
// @Description  Authenticate user and return JWT token
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Login credentials"
// @Success      200 {object} LoginResponse "Login successful"
// @Failure      400 {object} ErrorResponse "Invalid request body"
// @Failure      401 {object} ErrorResponse "Invalid credentials"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /auth/login [post]
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	const op = "handlers.AuthHandler.Login"

	var req models.LoginRequest
	if err := c.BodyParser(&req); err != nil {
		h.log.Error().
			Err(err).
			Str("op", op).
			Msg("failed to parse request body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if err := h.validate.Struct(&req); err != nil {
		h.log.Warn().
			Err(err).
			Str("op", op).
			Msg("validation failed")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "validation failed",
			"fields": h.formatValidationErrors(err),
		})
	}

	response, err := h.authService.Login(&req)
	if err != nil {
		// Check for known errors first
		if errors.Is(err, models.ErrInvalidCredentials) {
			h.log.Warn().
				Err(err).
				Str("op", op).
				Str("email", req.Email).
				Msg("invalid credentials")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid credentials",
			})
		}

		// All other errors are server errors - MUST log as Error!
		h.log.Error().
			Err(err).
			Str("op", op).
			Str("email", req.Email).
			Msg("login failed - server error")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "login failed",
		})
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", response.User.ID).
		Str("email", response.User.Email).
		Msg("user logged in successfully")

	return c.JSON(response)
}
