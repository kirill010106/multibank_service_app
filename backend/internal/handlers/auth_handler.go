package handlers

import (
	"errors"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/kirill010106/multibank_service_app/backend/internal/models"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/auth"
	"github.com/rs/zerolog"
)

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

// Register - POST /auth/register
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
		Msg("user registered succesfully")

	return c.Status(fiber.StatusCreated).JSON(response)

}

// Login - POST /auth/login

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
		h.log.Warn().Err(err).Str("op", op).Msg("validation failed")

		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":  "invalid credentials",
			"fields": h.formatValidationErrors(err),
		})
	}

	response, err := h.authService.Login(&req)
	if err != nil {
		h.log.Warn().
			Err(err).
			Str("op", op).
			Str("email", req.Email).
			Msg("login failed")
		if errors.Is(err, models.ErrInvalidCredentials) {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid credentials",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "login failed",
		})
	}

	h.log.Info().
		Str("op", op).
		Int("user_id", response.User.ID).
		Str("email", response.User.Email).
		Msg("user logged in succesfully")

	return c.JSON(response)
}
