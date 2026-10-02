// Package config reads and validates all runtime configuration from environment
// variables. Missing required variables cause an immediate, descriptive exit so
// operators see every problem at once rather than discovering them one at a time.
package config

import (
	"log/slog"
	"os"
	"strconv"
)

// Config holds all runtime configuration for the LinguaSpeed server.
// Values are read once at startup from environment variables via Load.
type Config struct {
	// DatabaseURL is the PostgreSQL DSN, e.g. postgres://user:pass@host:5432/db
	DatabaseURL string
	// RedisAddr is the Redis address, e.g. cache:6379
	RedisAddr string
	// JWTSecret is the HS256 signing secret for admin JWTs.
	JWTSecret string
	// ServerPort is the port the HTTP server listens on (default: "3000").
	ServerPort string
	// AdminUsername is the username for the seeded admin account.
	AdminUsername string
	// AdminPassword is the plaintext password for the seeded admin account.
	// It is hashed with bcrypt before storage and never logged.
	AdminPassword string
	// BasePoints is the scoring formula base points constant (default: 1000).
	BasePoints int
	// Penalty is the scoring formula penalty per failed attempt (default: 200).
	Penalty int
	// LogFormat controls slog output: "json" for structured JSON, "text" for human-readable.
	LogFormat string
}

// Load reads all configuration from environment variables and returns a Config.
// If any required variable is missing or invalid, Load logs all problems at once
// and exits with code 1 — this gives operators a complete picture rather than
// forcing them to restart repeatedly to discover each missing variable.
func Load() *Config {
	var missing []string

	required := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}

	optional := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	optionalInt := func(key string, fallback int) int {
		s := os.Getenv(key)
		if s == "" {
			return fallback
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			// why: treat an unparseable int as a missing-value error so operators
			// are told exactly which variable has a bad value rather than silently
			// falling back to the default.
			missing = append(missing, key+"(must be integer, got: "+s+")")
			return fallback
		}
		return n
	}

	// Build DATABASE_URL from individual DB_* parts so the compose file can
	// supply user/password/name as separate variables while the app uses a
	// single DSN internally.
	dbUser := required("DB_USER")
	dbPassword := required("DB_PASSWORD")
	dbName := required("DB_NAME")
	dbHost := optional("DB_HOST", "db")
	dbPort := optional("DB_PORT", "5432")

	cfg := &Config{
		RedisAddr:     required("REDIS_ADDR"),
		JWTSecret:     required("JWT_SECRET"),
		AdminUsername: required("ADMIN_USERNAME"),
		AdminPassword: required("ADMIN_PASSWORD"),
		ServerPort:    optional("SERVER_PORT", "3000"),
		LogFormat:     optional("LOG_FORMAT", "text"),
		BasePoints:    optionalInt("BASE_POINTS", 1000),
		Penalty:       optionalInt("PENALTY", 200),
	}

	if len(missing) > 0 {
		// why: log all missing variables in one message so the operator fixes
		// everything in a single restart cycle rather than discovering them one
		// by one.
		slog.Error("missing required environment variables", "variables", missing)
		os.Exit(1)
	}

	// Assemble the DSN after the missing-variable check so we never construct a
	// partial DSN that would produce a confusing connection error downstream.
	cfg.DatabaseURL = "postgres://" + dbUser + ":" + dbPassword + "@" + dbHost + ":" + dbPort + "/" + dbName

	return cfg
}
