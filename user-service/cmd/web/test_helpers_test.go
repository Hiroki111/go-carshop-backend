package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/oauth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/service"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Generating an RSA key is slow, so it is created once per test binary.
var testKey = mustGenerateKey()

func mustGenerateKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

func setupTestApp(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	svc, db := newTestService(t)

	return routes(handler.NewHandler(svc)), db
}

func newTestService(t *testing.T) (*service.Service, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		// Without this, a unique-index violation isn't reported as
		// gorm.ErrDuplicatedKey, which the repository relies on.
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	repo := repository.NewRepository(db)
	require.NoError(t, repo.Migrate())

	clients, err := oauth.NewRegistry()
	require.NoError(t, err)

	return service.NewService(repo, testKey, clients), db
}

func executeRequest(t *testing.T, app http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	switch b := body.(type) {
	case nil:
	case string:
		buf.WriteString(b)
	default:
		require.NoError(t, json.NewEncoder(&buf).Encode(b))
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	return rec
}

func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()

	var v T
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&v))
	return v
}
