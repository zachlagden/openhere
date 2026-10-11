package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/zachlagden/openhere/internal/store"
)

const databaseURLVariable = "TEST_DATABASE_URL"

func Each(t *testing.T, fn func(t *testing.T, db store.Store)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) {
		fn(t, openSQLite(t))
	})
	t.Run("postgres", func(t *testing.T) {
		base := os.Getenv(databaseURLVariable)
		if base == "" {
			t.Skip(databaseURLVariable + " is not set")
		}
		fn(t, openPostgresSchema(t, base))
	})
}

func openSQLite(t *testing.T) store.Store {
	t.Helper()
	db, err := store.Open(context.Background(), store.Options{DataDir: t.TempDir()})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { closeStore(t, db) })
	migrate(t, db)
	return db
}

func openPostgresSchema(t *testing.T, base string) store.Store {
	t.Helper()
	ctx := context.Background()
	schema := "test_" + randomSuffix(t)

	admin, err := store.Open(ctx, store.Options{DatabaseURL: base})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { closeStore(t, admin) })
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
	})

	scoped, err := withSearchPath(base, schema)
	if err != nil {
		t.Fatalf("scope database url: %v", err)
	}
	db, err := store.Open(ctx, store.Options{DatabaseURL: scoped})
	if err != nil {
		t.Fatalf("open scoped postgres: %v", err)
	}
	t.Cleanup(func() { closeStore(t, db) })
	migrate(t, db)
	return db
}

func withSearchPath(base, schema string) (string, error) {
	parsed, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func migrate(t *testing.T, db store.Store) {
	t.Helper()
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func closeStore(t *testing.T, db store.Store) {
	t.Helper()
	if err := db.Close(); err != nil {
		t.Errorf("close store: %v", err)
	}
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(buf)
}

func InsertUser(t *testing.T, db store.Store, email string) string {
	t.Helper()
	id := randomSuffix(t) + randomSuffix(t) + randomSuffix(t)[:8]
	now := time.Now().UTC().Truncate(time.Second)
	_, err := db.ExecContext(context.Background(),
		"INSERT INTO users (id, email, password_hash, is_root, created_at, updated_at) VALUES ($1, $2, $3, FALSE, $4, $4)",
		id, email, "not-a-real-hash", now)
	if err != nil {
		t.Fatalf("insert user %s: %v", email, err)
	}
	return id
}
