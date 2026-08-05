package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the fully resolved application configuration, derived from the
// environment with safe local defaults and fail-fast validation. The
// notification service is a pure consumer: it has no HTTP server and no auth.
type Config struct {
	ServiceName string
	Environment string
	MetricsAddr string
	Database    DatabaseConfig
	Kafka       KafkaConfig
	Consumer    ConsumerConfig
	Telemetry   TelemetryConfig
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
}

type KafkaConfig struct {
	Brokers []string
}

type ConsumerConfig struct {
	GroupID string
	Topics  []string
}

type TelemetryConfig struct {
	OTLPEndpoint string
	SampleRatio  float64
}

// Load reads configuration from the environment and validates it.
func Load() (Config, error) {
	cfg := Config{
		ServiceName: env("SERVICE_NAME", "notification-service"),
		Environment: env("ENVIRONMENT", "development"),
		MetricsAddr: env("METRICS_ADDR", ":9090"),
		Database: DatabaseConfig{
			URL:             env("DATABASE_URL", "postgres://notification:notification@localhost:5436/notification?sslmode=disable"),
			MaxConns:        int32(envInt("DB_MAX_CONNS", 10)),
			MinConns:        int32(envInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime: envDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		},
		Kafka: KafkaConfig{
			Brokers: envList("KAFKA_BROKERS", []string{"localhost:29092"}),
		},
		Consumer: ConsumerConfig{
			GroupID: env("CONSUMER_GROUP_ID", "notification"),
			Topics:  envList("CONSUMER_TOPICS", []string{"orders.events", "payments.events", "fulfillment.events"}),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			SampleRatio:  envFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
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
	if len(c.Consumer.Topics) == 0 {
		return fmt.Errorf("CONSUMER_TOPICS is required")
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

func envDuration(key string, fallback time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
