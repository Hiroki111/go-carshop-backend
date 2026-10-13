// Package testutil holds helpers shared by tests. It is imported only from
// _test.go files.
package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// PostgresDSNEnvVar names the environment variable that points the tests at a
// real Postgres, for example:
//
//	postgres://car_service_user:password@localhost:5432/car_service?sslmode=disable
const PostgresDSNEnvVar = "TEST_POSTGRES_DSN"

// NewPostgresDB returns a connection to a brand-new, empty schema of the
// Postgres that TEST_POSTGRES_DSN points at, and drops the schema when the test
// ends. Tables the test creates live only in that schema, so the data in the
// public schema (the one the app uses) is never read or modified.
//
// The test is skipped when the variable is not set, which keeps a plain
// `go test ./...` working without a database.
//
// Use this, not SQLite, for tests about concurrency: an in-memory SQLite
// database serialises every write, which hides exactly the races we want to see.
func NewPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv(PostgresDSNEnvVar)
	if dsn == "" {
		t.Skipf("%s is not set; skipping the test that needs a real Postgres", PostgresDSNEnvVar)
	}

	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("failed to connect to Postgres (%s): %v", PostgresDSNEnvVar, err)
	}

	schema := "test_" + randomHex(t, 8)
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		closeDB(admin)
		t.Fatalf("failed to create schema %s: %v", schema, err)
	}

	db, err := gorm.Open(
		postgres.Open(withSearchPath(t, dsn, schema)),
		&gorm.Config{
			TranslateError: true,
			Logger:         logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		closeDB(admin)
		t.Fatalf("failed to connect to schema %s: %v", schema, err)
	}

	t.Cleanup(func() {
		// Close the test connections first so nothing holds a lock on the schema.
		closeDB(db)
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("failed to drop schema %s: %v", schema, err)
		}
		closeDB(admin)
	})

	return db
}

// withSearchPath makes every connection created from dsn use schema first, so
// that unqualified table names (and AutoMigrate) land in it.
func withSearchPath(t *testing.T, dsn, schema string) string {
	t.Helper()

	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", PostgresDSNEnvVar, err)
		}
		q := u.Query()
		q.Set("search_path", schema)
		u.RawQuery = q.Encode()
		return u.String()
	}

	// key=value form: "host=localhost user=... dbname=..."
	return dsn + " search_path=" + schema
}

func randomHex(t *testing.T, n int) string {
	t.Helper()

	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatalf("failed to generate random bytes: %v", err)
	}
	return hex.EncodeToString(b)
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
