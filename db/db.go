// Package db opens the database behind an Ent client. SQLite uses the pure-Go
// modernc.org/sqlite driver, so the binary stays cgo-free.
package db

import (
	"database/sql"
	"fmt"
	"strings"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

const sqlitePragmas = "_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"

// Open connects to url: "sqlite://path/to/file.db" (or "sqlite://file:name?mode=memory&cache=shared")
// or "postgres://user@host/db". Pass the result to ent.Driver.
func Open(url string) (*entsql.Driver, error) {
	switch {
	case strings.HasPrefix(url, "sqlite://"):
		dsn := strings.TrimPrefix(url, "sqlite://")
		if !strings.Contains(dsn, "foreign_keys") {
			sep := "?"
			if strings.Contains(dsn, "?") {
				sep = "&"
			}
			dsn += sep + sqlitePragmas
		}
		return open("sqlite", dialect.SQLite, dsn)
	case strings.HasPrefix(url, "postgres://"), strings.HasPrefix(url, "postgresql://"):
		return open("pgx", dialect.Postgres, url)
	}
	return nil, fmt.Errorf("db: unsupported database url %q (want sqlite:// or postgres://)", redact(url))
}

func open(driver, d, dsn string) (*entsql.Driver, error) {
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	return entsql.OpenDB(d, db), nil
}

// redact keeps credentials out of error messages: only the scheme is shown.
func redact(url string) string {
	if i := strings.Index(url, "://"); i >= 0 {
		return url[:i+3] + "..."
	}
	return "..."
}
