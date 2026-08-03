package postgres

import "embed"

// MigrationsFS holds the SQL migrations embedded into the binary so deployments
// carry their schema with them.
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS
