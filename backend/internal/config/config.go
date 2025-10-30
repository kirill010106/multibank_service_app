package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
)

type Config struct {
	Env          string `yaml:"env" env:"ENV" env-default:"prod"`
	StorageURL   string `yaml:"storage_path" env:"DATABASE_URL" env_required:"true"`
	HTTPServer   `yaml:"http_server"`
	ClientID     string      `yaml:"client_id" env:"CLIENT_ID" env-required:"true"`
	ClientSecret string      `yaml:"client_secret" env:"CLIENT_SECRET" env-required:"true"`
	VirtualBank  VirtualBank `yaml:"virtual_bank"`
	JWTSecret    string      `yaml:"jwt_secret" env:"JWT_SECRET" env-required:"true"`
}

type HTTPServer struct {
	Address     string        `yaml:"address" env-default:"localhost:8080"`
	Timeout     time.Duration `yaml:"timeout" env-default:"4s"`
	IdleTimeout time.Duration `yaml:"idle_timeout" env-default:"60s"`
}

type VirtualBank struct {
	BaseURL string `yaml:"base_url" env_required:"true"`
}

// GetHost возвращает только хост из адреса (localhost или 0.0.0.0)
func (h *HTTPServer) GetHost() string {
	// Извлекаем хост из address (например, "localhost:8080" -> "localhost")
	if len(h.Address) > 0 {
		for i, c := range h.Address {
			if c == ':' {
				return h.Address[:i]
			}
		}
	}
	return "localhost"
}

// GetPort возвращает только порт из адреса
func (h *HTTPServer) GetPort() string {
	// Извлекаем порт из address (например, "localhost:8080" -> "8080")
	for i, c := range h.Address {
		if c == ':' {
			return h.Address[i+1:]
		}
	}
	return "8080"
}

func Init() {
	if err := godotenv.Load(); err != nil {
		log.Fatalln("No .env file")
	}
	log.Println(".env file loaded")
}

func MustLoad() *Config {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		log.Fatal("CONFIG_PATH is not set")
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("config file does not exist: %s", configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		log.Fatalf("cannot read config %s", err)
	}

	if cfg.StorageURL == "" {
		log.Fatal("DATABASE_URL is not set")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is not set")
	}
	return &cfg
}

type LogConfig struct {
	Level  int
	Format string
}

func NewLogConfig() *LogConfig {
	level, err := strconv.Atoi(os.Getenv("LOG_LEVEL"))
	if err != nil {
		level = 1 // default: debug
	}

	format := os.Getenv("LOG_FORMAT")
	if format == "" {
		format = "console"
	}

	return &LogConfig{
		Level:  level,
		Format: format,
	}
}
