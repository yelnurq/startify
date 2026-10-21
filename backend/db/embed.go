// Package db embeds the SQL migrations so binaries are self-contained.
package db

import "embed"

// Migrations holds the goose migrations in migrations/*.sql.
//
//go:embed migrations/*.sql
var Migrations embed.FS
