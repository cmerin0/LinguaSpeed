// Package config_test contains unit tests for environment variable loading.
package config_test

import (
	"os"
	"testing"

	"linguaspeed/internal/config"
)

// TestLoad_defaults verifies that optional variables fall back to expected defaults.
func TestLoad_defaults(t *testing.T) {
	// Set all required vars; leave optional ones unset to test defaults.
	os.Setenv("DB_USER", "u")
	os.Setenv("DB_PASSWORD", "p")
	os.Setenv("DB_NAME", "db")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("JWT_SECRET", "secret")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "pass")
	// Intentionally NOT setting SERVER_PORT, BASE_POINTS, PENALTY, LOG_FORMAT.
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("BASE_POINTS")
	os.Unsetenv("PENALTY")
	os.Unsetenv("LOG_FORMAT")

	cfg := config.Load()

	if cfg.ServerPort != "3000" {
		t.Errorf("expected ServerPort=3000, got %q", cfg.ServerPort)
	}
	if cfg.BasePoints != 1000 {
		t.Errorf("expected BasePoints=1000, got %d", cfg.BasePoints)
	}
	if cfg.Penalty != 200 {
		t.Errorf("expected Penalty=200, got %d", cfg.Penalty)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("expected LogFormat=text, got %q", cfg.LogFormat)
	}
}

// TestLoad_dsnConstruction verifies the DATABASE_URL is built correctly from parts.
func TestLoad_dsnConstruction(t *testing.T) {
	os.Setenv("DB_USER", "cmerino")
	os.Setenv("DB_PASSWORD", "anypass")
	os.Setenv("DB_NAME", "linguaspeed")
	os.Setenv("DB_HOST", "myhost")
	os.Setenv("DB_PORT", "5433")
	os.Setenv("REDIS_ADDR", "localhost:6379")
	os.Setenv("JWT_SECRET", "secret")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "pass")

	cfg := config.Load()
	want := "postgres://cmerino:anypass@myhost:5433/linguaspeed"
	if cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL: got %q, want %q", cfg.DatabaseURL, want)
	}
}
