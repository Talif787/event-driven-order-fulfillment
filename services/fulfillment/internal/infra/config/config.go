package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved application configuration, derived from the
// environment with safe local defaults and fail-fast validation.
type Config struct {
	ServiceName        string
	Environment        string
	HTTPAddr           string
	CORSAllowedOrigins []string
	MetricsAddr        string
	Database           DatabaseConfig
	Kafka              KafkaConfig
	Consumer           ConsumerConfig
	Telemetry          TelemetryConfig
	Auth               AuthConfig
	Timeouts           TimeoutConfig
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type KafkaConfig struct {
	Brokers     []string
	EventsTopic string
}

type ConsumerConfig struct {
	GroupID         string
	Topics          []string
	DeadLetterTopic string
}

type TelemetryConfig struct {
	OTLPEndpoint string
	SampleRatio  float64
}

type AuthConfig struct {
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
	Enabled     bool
}

type TimeoutConfig struct {
	ReadHeader time.Duration
	Read       time.Duration
	Write      time.Duration
	Idle       time.Duration
	Shutdown   time.Duration
}

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	cfg := Config{
		ServiceName:        env("SERVICE_NAME", "fulfillment-service"),
		Environment:        env("ENVIRONMENT", "development"),
		HTTPAddr:           env("HTTP_ADDR", ":8083"),
		CORSAllowedOrigins: envList("CORS_ALLOWED_ORIGINS", nil),
		MetricsAddr:        env("METRICS_ADDR", ":9090"),
		Database: DatabaseConfig{
			URL:             env("DATABASE_URL", "postgres://fulfillment:fulfillment@localhost:5435/fulfillment?sslmode=disable"),
			MaxConns:        int32(envInt("DB_MAX_CONNS", 20)),
			MinConns:        int32(envInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime: envDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		},
		Kafka: KafkaConfig{
			Brokers:     envList("KAFKA_BROKERS", []string{"localhost:29092"}),
			EventsTopic: env("FULFILLMENT_EVENTS_TOPIC", "fulfillment.events"),
		},
		Consumer: ConsumerConfig{
			GroupID:         env("CONSUMER_GROUP_ID", "fulfillment"),
			Topics:          envList("CONSUMER_TOPICS", []string{"orders.events"}),
			DeadLetterTopic: env("DEAD_LETTER_TOPIC", "dead-letter"),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			SampleRatio:  envFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
		},
		Auth: AuthConfig{
			JWTSecret:   env("AUTH_JWT_SECRET", ""),
			JWTIssuer:   env("AUTH_JWT_ISSUER", "fulfillment-service"),
			JWTAudience: env("AUTH_JWT_AUDIENCE", "fulfillment-api"),
			Enabled:     envBool("AUTH_ENABLED", false),
		},
		Timeouts: TimeoutConfig{
			ReadHeader: envDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			Read:       envDuration("HTTP_READ_TIMEOUT", 15*time.Second),
			Write:      envDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			Idle:       envDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			Shutdown:   envDuration("HTTP_SHUTDOWN_TIMEOUT", 20*time.Second),
		},
	}
	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if c.Database.URL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.Database.MinConns < 0 || c.Database.MaxConns < c.Database.MinConns {
		return fmt.Errorf("invalid DB connection bounds: min=%d max=%d", c.Database.MinConns, c.Database.MaxConns)
	}
	if len(c.Kafka.Brokers) == 0 {
		return fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.Auth.Enabled && c.Auth.JWTSecret == "" {
		return fmt.Errorf("AUTH_JWT_SECRET is required when AUTH_ENABLED=true")
	}
	if c.Telemetry.SampleRatio < 0 || c.Telemetry.SampleRatio > 1 {
		return fmt.Errorf("OTEL_TRACES_SAMPLER_RATIO must be within [0,1]")
	}
	return nil
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		parts := strings.Split(v, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				out = append(out, s)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v, ok := os.LookupEnv(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
