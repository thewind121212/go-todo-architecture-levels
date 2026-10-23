// Package migrations embeds the SQL schema files into the binary,
// so the app can migrate Postgres at startup without files on disk.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
