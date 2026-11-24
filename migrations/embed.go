package migrations

import "embed"

// FS contains versioned schema migrations.
//
//go:embed *.sql
var FS embed.FS
