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
