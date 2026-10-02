// Package migrations embeds the SQL migration files so the compiled binary is
// fully self-contained — no migration files need to be present on disk at runtime.
package migrations

import "embed"

// FS holds all SQL migration files embedded at compile time.
// It is consumed by golang-migrate via iofs.New(migrations.FS, ".") in main.
//
//go:embed *.sql
var FS embed.FS
