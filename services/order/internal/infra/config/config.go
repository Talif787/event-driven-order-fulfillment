package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config is the fully resolved application configuration. All values derive
// from the environment (Twelve-Factor), with safe defaults for local use and
// explicit validation so the process fails fast on misconfiguration.
type Config struct {
	ServiceName string
	Environment string
	HTTPAddr    string
	Database    DatabaseConfig
	Telemetry   TelemetryConfig
	Auth        AuthConfig
	Timeouts    TimeoutConfig
}

type DatabaseConfig struct {
	URL             string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
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
		ServiceName: env("SERVICE_NAME", "order-service"),
		Environment: env("ENVIRONMENT", "development"),
		HTTPAddr:    env("HTTP_ADDR", ":8080"),
		Database: DatabaseConfig{
			URL:             env("DATABASE_URL", "postgres://order:order@localhost:5432/order?sslmode=disable"),
			MaxConns:        int32(envInt("DB_MAX_CONNS", 20)),
			MinConns:        int32(envInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime: envDuration("DB_MAX_CONN_LIFETIME", time.Hour),
		},
		Telemetry: TelemetryConfig{
			OTLPEndpoint: env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			SampleRatio:  envFloat("OTEL_TRACES_SAMPLER_RATIO", 1.0),
		},
		Auth: AuthConfig{
			JWTSecret:   env("AUTH_JWT_SECRET", ""),
			JWTIssuer:   env("AUTH_JWT_ISSUER", "order-service"),
			JWTAudience: env("AUTH_JWT_AUDIENCE", "order-api"),
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
