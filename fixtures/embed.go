package fixtures

import "embed"

// FS contains idempotent demo data.
//
//go:embed *.sql
var FS embed.FS
