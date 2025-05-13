package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the service
type Config struct {
	Server     ServerConfig     `json:"server"`
	Redis      RedisConfig      `json:"redis"`
	Auth       AuthConfig       `json:"auth"`
	Kafka      KafkaConfig      `json:"kafka"`
	Logger     LoggerConfig     `json:"logger"`
	Monitoring MonitoringConfig `json:"monitoring"`
}

// ServerConfig holds configurations for the server
type ServerConfig struct {
	HTTPPort       int           `json:"http_port"`
	GRPCPort       int           `json:"grpc_port"`
	WebSocketPort  int           `json:"websocket_port"`
	ReadTimeout    time.Duration `json:"read_timeout"`
	WriteTimeout   time.Duration `json:"write_timeout"`
	AllowedOrigins []string      `json:"allowed_origins"`
}

// RedisConfig holds configuration for Redis
type RedisConfig struct {
	URL      string        `json:"url"`
	Password string        `json:"password"`
	DB       int           `json:"db"`
	Timeout  time.Duration `json:"timeout"`
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret             string        `json:"jwt_secret"`
	JWTExpiryDuration     time.Duration `json:"jwt_expiry_duration"`
	SessionExpiryDuration time.Duration `json:"session_expiry_duration"`
	OAuth                 OAuthConfig   `json:"oauth"`
}

// OAuthConfig holds OAuth configurations
type OAuthConfig struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
}

// KafkaConfig holds Kafka configurations
type KafkaConfig struct {
	Enabled       bool     `json:"enabled"`
	Brokers       []string `json:"brokers"`
	Topic         string   `json:"topic"`
	ConsumerGroup string   `json:"consumer_group"`
}

// LoggerConfig holds logger configurations
type LoggerConfig struct {
	Level      string `json:"level"`
	OutputPath string `json:"output_path"`
	Format     string `json:"format"`
}

// MonitoringConfig holds monitoring configurations
type MonitoringConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Path    string `json:"path"`
}

// LoadConfig loads configuration from environment variables and config file
func LoadConfig(configPath string) (*Config, error) {
	// Load .env file if it exists
	_ = godotenv.Load()

	// Set default configuration
	cfg := &Config{
		Server: ServerConfig{
			HTTPPort:       8080,
			GRPCPort:       9090,
			WebSocketPort:  8081,
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			AllowedOrigins: []string{"*"},
		},
		Redis: RedisConfig{
			URL:      "localhost:6379",
			Password: "",
			DB:       0,
			Timeout:  5 * time.Second,
		},
		Auth: AuthConfig{
			JWTSecret:             getEnvOrDefault("JWT_SECRET", "your-secret-key-change-in-production"),
			JWTExpiryDuration:     24 * time.Hour,
			SessionExpiryDuration: 72 * time.Hour,
		},
		Kafka: KafkaConfig{
			Enabled:       false,
			Brokers:       []string{"localhost:9092"},
			Topic:         "session-events",
			ConsumerGroup: "kratos-session-service",
		},
		Logger: LoggerConfig{
			Level:      "info",
			OutputPath: "stdout",
			Format:     "json",
		},
		Monitoring: MonitoringConfig{
			Enabled: true,
			Port:    9100,
			Path:    "/metrics",
		},
	}

	// If config file exists, load it
	if configPath != "" {
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("error opening config file: %w", err)
		}
		defer file.Close()

		decoder := json.NewDecoder(file)
		if err := decoder.Decode(cfg); err != nil {
			return nil, fmt.Errorf("error decoding config file: %w", err)
		}
	}

	// Override with environment variables
	overrideConfigFromEnv(cfg)

	return cfg, nil
}

// Helper function to get environment variable with a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Override config values from environment variables
func overrideConfigFromEnv(cfg *Config) {
	// Server config
	if port := os.Getenv("HTTP_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.HTTPPort)
	}
	if port := os.Getenv("GRPC_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.GRPCPort)
	}
	if port := os.Getenv("WEBSOCKET_PORT"); port != "" {
		fmt.Sscanf(port, "%d", &cfg.Server.WebSocketPort)
	}

	// Redis config
	if url := os.Getenv("REDIS_URL"); url != "" {
		cfg.Redis.URL = url
	}
	if password := os.Getenv("REDIS_PASSWORD"); password != "" {
		cfg.Redis.Password = password
	}
	if db := os.Getenv("REDIS_DB"); db != "" {
		fmt.Sscanf(db, "%d", &cfg.Redis.DB)
	}

	// Auth config
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		cfg.Auth.JWTSecret = secret
	}

	// Kafka config
	if enabled := os.Getenv("KAFKA_ENABLED"); enabled == "true" {
		cfg.Kafka.Enabled = true
	}
	if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
		// Parse comma-separated brokers
		cfg.Kafka.Brokers = []string{brokers}
	}
	if topic := os.Getenv("KAFKA_TOPIC"); topic != "" {
		cfg.Kafka.Topic = topic
	}

	// Logger config
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		cfg.Logger.Level = level
	}
	if output := os.Getenv("LOG_OUTPUT"); output != "" {
		cfg.Logger.OutputPath = output
	}
}
