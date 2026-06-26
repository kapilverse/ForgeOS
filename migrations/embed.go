// Package migrations embeds the SQL migration files so the compiled ForgeOS
// binary is self-contained (no external migration files needed at runtime).
package migrations

import "embed"

// FS holds all *.sql migration files from this directory.
//
//go:embed *.sql
var FS embed.FS
