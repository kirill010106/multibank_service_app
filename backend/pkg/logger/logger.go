package logger

import (
	"fmt"
	"os"

	"github.com/kirill010106/multibank_service_app/backend/internal/config"
	"github.com/rs/zerolog"
)

func NewLogger(config *config.LogConfig) *zerolog.Logger {
	var logger zerolog.Logger
	if config.Format == "json" {
		logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	} else {
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
		logger = zerolog.New(consoleWriter).With().Timestamp().Logger()

	}
	return &logger
}

// LogServerStart логирует информацию о запуске сервера
func LogServerStart(logger *zerolog.Logger, cfg *config.Config) {
	host := cfg.HTTPServer.GetHost()
	port := cfg.HTTPServer.GetPort()

	if cfg.Env == "local" {
		fmt.Println("\n" + "═════════════════════════════════════════════════")
		fmt.Println("🏦  VTB Multibank API Server")
		fmt.Println("═════════════════════════════════════════════════")
		fmt.Printf("📍 Environment:  %s\n", cfg.Env)
		fmt.Printf("🌐 Address:      http://%s:%s\n", host, port)
		fmt.Printf("💚 Health:       http://%s:%s/health\n", host, port)
		fmt.Printf("📚 API:          http://%s:%s/api/v1\n", host, port)
		fmt.Println("═════════════════════════════════════════════════")
		fmt.Println("Press CTRL+C to stop the server")
	}

	// Структурированный лог
	logger.Info().
		Str("env", cfg.Env).
		Str("address", cfg.HTTPServer.Address).
		Str("host", host).
		Str("port", port).
		Msg("Server starting")
}

// LogServerListening логирует успешный запуск сервера
func LogServerListening(logger *zerolog.Logger, address string) {
	logger.Info().
		Str("address", address).
		Msg("Server is listening")
}
