package config

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultJWTSecret = "your-secret-key"

type Config struct {
	Port                 string
	DatabaseURL          string
	JWTSecret            string
	Environment          string
	TrustProxy           bool          // if true, rate limiting uses X-Real-IP / X-Forwarded-For for client IP (set when behind a trusted reverse proxy)
	TokenExpiry          time.Duration // JWT token lifetime (e.g. 24h)
	RevocationTolerance  time.Duration // tolerance when comparing token iat to token_valid_after (DB precision, timezone)
}

func Load() *Config {
	_ = godotenv.Load()
	return &Config{
		Port:                getEnv("PORT", "8080"),
		DatabaseURL:         getEnv("DATABASE_URL", ""),
		JWTSecret:           getEnv("JWT_SECRET", defaultJWTSecret),
		Environment:        getEnv("ENVIRONMENT", "development"),
		TrustProxy:          getEnv("TRUST_PROXY", "") == "true" || getEnv("TRUST_PROXY", "") == "1",
		TokenExpiry:         getEnvDuration("JWT_EXPIRY", 24*time.Hour),
		RevocationTolerance: getEnvDuration("REVOCATION_TOLERANCE", 2*time.Second),
	}
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	s := getEnv(key, "")
	if s == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return defaultVal
	}
	return d
}

// Validate returns an error if config is unsafe for the current environment (e.g. missing required values in production).
func (c *Config) Validate() error {
	if strings.ToLower(c.Environment) != "production" {
		return nil
	}
	if c.JWTSecret == "" || c.JWTSecret == defaultJWTSecret {
		return errors.New("production requires JWT_SECRET to be set and not the default value")
	}
	if c.DatabaseURL == "" {
		return errors.New("production requires DATABASE_URL to be set")
	}
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

