package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/kirill010106/multibank_service_app/backend/internal/services/auth"
	"github.com/rs/zerolog"
)

func JWTAuth(jwtManager *auth.JWTManager, log zerolog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		const op = "middleware.JWTAuth"

		authHeader := c.Get("Authorization")
		if authHeader == "" {
			log.Warn().
				Str("op", op).
				Str("path", c.Path()).
				Msg("missing Authorization header")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "unauthorized",
			})
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			log.Warn().
				Str("op", op).
				Str("path", c.Path()).
				Msg("invalid Authorization header format")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid authorization header format",
			})
		}

		tokenString := parts[1]

		claims, err := jwtManager.ValidateToken(tokenString)
		if err != nil {
			log.Warn().
				Err(err).
				Str("op", op).
				Msg("invalid token")
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "invalid token",
			})
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("email", claims.Email)

		log.Debug().
			Str("op", op).
			Int("user_id", claims.UserID).
			Str("email", claims.Email).
			Str("path", c.Path()).
			Msg("token validated succesfully")

		return c.Next()
	}
}
