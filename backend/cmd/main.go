// @title           VTB Multibank Service API
// @version         1.0
// @description     Multibank aggregation service for VTB Hackathon. Use /auth/register or /auth/login to get JWT token, then click "Authorize" button and paste token.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@vtbmultibank.ru

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token in the format: Bearer {token}
package main

import (
	"os"

	"github.com/kirill010106/multibank_service_app/backend/internal/app"
	"github.com/kirill010106/multibank_service_app/backend/internal/config"
	"github.com/kirill010106/multibank_service_app/backend/pkg/logger"
	"github.com/rs/zerolog"
)

func main() {
	// Initialize configuration
	config.Init()
	cfg := config.MustLoad()

	// Initialize logger
	logConfig := config.NewLogConfig()
	log := logger.NewLogger(logConfig)
	zerolog.SetGlobalLevel(zerolog.Level(logConfig.Level))

	log.Info().
		Str("env", cfg.Env).
		Str("address", cfg.HTTPServer.Address).
		Msg("starting VTB Multibank API")

	// Create and initialize application
	application, err := app.New(cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize application")
	}
	defer application.Close()

	// Run the server
	if err := application.Run(); err != nil {
		log.Fatal().Err(err).Msg("failed to run application")
	}

	os.Exit(0)
}
