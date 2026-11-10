// Package testdb gives each test its own Postgres schema loaded from db/schema.sql.
//
// Tests that need a database call testdb.New(t). They are skipped unless TEST_DATABASE_URL is set,
// so `go test ./...` still passes on a laptop without Postgres. backend/scripts/test-on-server.sh
// runs the whole suite against a throwaway Postgres container.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New returns a pool whose search_path is a fresh schema with the app tables. The schema is dropped when the test ends.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	b := make([]byte, 6)
	rand.Read(b)
	schema := "t_" + hex.EncodeToString(b)

	admin, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("testdb: connect: %v", err)
	}
	defer admin.Close(ctx)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("testdb: create schema: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), url)
		if err != nil {
			return
		}
		defer c.Close(context.Background())
		c.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatalf("testdb: parse url: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("testdb: pool: %v", err)
	}
	t.Cleanup(pool.Close)

	sql, err := os.ReadFile(schemaPath())
	if err != nil {
		t.Fatalf("testdb: read schema: %v", err)
	}
	// Simple protocol: the schema file has several statements.
	if _, err := pool.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
		t.Fatalf("testdb: apply schema: %v", err)
	}
	return pool
}

func schemaPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "schema.sql")
}
