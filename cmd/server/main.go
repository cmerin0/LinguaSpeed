// Package main is the entry point for the LinguaSpeed HTTP server.
// It loads configuration, runs database migrations, seeds the admin user,
// connects to Redis, and starts the HTTP listener.
package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	migratepg "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq" // postgres driver for database/sql (used by golang-migrate)
	"golang.org/x/crypto/bcrypt"

	"linguaspeed/internal/cache"
	"linguaspeed/internal/config"
	"linguaspeed/internal/db"
	"linguaspeed/internal/db/migrations"
)

func main() {
	// Load and validate all environment variables up front.
	// config.Load exits with code 1 if any required variable is missing,
	// logging all missing names at once.
	cfg := config.Load()

	// Configure structured logging based on LOG_FORMAT env var.
	// why: JSON format is machine-parseable for production log aggregators;
	// text format is human-readable for local development.
	setupLogger(cfg.LogFormat)

	ctx := context.Background()

	// --- Database ---

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run all pending SQL migrations before accepting any traffic.
	// why: migrations must complete before the server starts so that handlers
	// never operate against a stale or incomplete schema.
	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	// Seed the initial admin user if one does not already exist.
	if err := seedAdmin(ctx, pool, cfg); err != nil {
		slog.Error("admin seed failed", "error", err)
		os.Exit(1)
	}

	// --- Cache ---

	redisClient, err := cache.New(ctx, cfg.RedisAddr)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	slog.Info("server started", "port", cfg.ServerPort)

	// TODO: wire chi router and HTTP server (task 9.5)
	_ = redisClient
}

// setupLogger configures the global slog logger.
// format "json" produces machine-parseable output; any other value produces
// human-readable text output suitable for local development.
func setupLogger(format string) {
	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, nil)
	} else {
		handler = slog.NewTextHandler(os.Stdout, nil)
	}
	slog.SetDefault(slog.New(handler))
}

// runMigrations applies all pending SQL migrations using the embedded migration
// files. It returns nil if migrations are already up to date (migrate.ErrNoChange
// is not treated as an error). It returns an error if any migration fails, which
// causes the caller to halt startup immediately.
func runMigrations(databaseURL string) error {
	// why: iofs.New reads migration files from the embedded filesystem rather
	// than from disk, so the binary is fully self-contained.
	srcDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return err
	}

	// why: We open a plain sql.DB connection just for golang-migrate, separate
	// from the pgxpool used by repositories. golang-migrate does not support
	// pgxpool natively; using database/sql with lib/pq is the recommended approach.
	sqlDB, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()

	dbDriver, err := migratepg.WithInstance(sqlDB, &migratepg.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", srcDriver, "postgres", dbDriver)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}

	slog.Info("database migrations applied")
	return nil
}

// seedAdmin creates the first admin user if none exists.
// If the user already exists, seed is skipped and an info log is emitted.
// why: the admin user is created by a seed script rather than a public signup
// form, so it must exist before the server accepts admin API requests.
func seedAdmin(ctx context.Context, pool *pgxpool.Pool, cfg *config.Config) error {
	// Check whether any admin user already exists.
	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		slog.Info("admin seed skipped: admin user already exists")
		return nil
	}

	// Hash the password with bcrypt at cost 12.
	// why: cost 12 is the minimum recommended value balancing security
	// (resistance to brute-force) and startup time (< 1 second per hash).
	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), 12)
	if err != nil {
		return err
	}

	_, err = pool.Exec(ctx,
		"INSERT INTO users (username, hashed_password, role) VALUES ($1, $2, 'admin')",
		cfg.AdminUsername, string(hashed),
	)
	if err != nil {
		return err
	}

	slog.Info("admin seed complete", "username", cfg.AdminUsername)
	return nil
}
