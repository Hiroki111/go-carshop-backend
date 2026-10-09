package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// trustedKey is the key pair whose public half the app under test trusts.
// Generating an RSA key is slow, so it is created once per test binary.
var trustedKey = mustGenerateKey()

func mustGenerateKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

// newTestKey returns a key the app under test does NOT trust.
func newTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	return mustGenerateKey()
}

func signToken(t *testing.T, key *rsa.PrivateKey, role auth.UserRole, expiresAt time.Time) string {
	t.Helper()

	claims := auth.Claims{
		UserID:           1,
		Role:             role,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)},
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

// tokenFor returns a valid, unexpired token for the given role that the app
// under test accepts.
func tokenFor(t *testing.T, role auth.UserRole) string {
	t.Helper()
	return signToken(t, trustedKey, role, time.Now().Add(time.Hour))
}

func setupTestApp(t *testing.T) (http.Handler, *gorm.DB) {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		TranslateError: true,
		Logger:         logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}

	repo := repository.NewRepository(db)

	if err := repo.Migrate(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	carsCache := cache.NewNoopCarsCache()
	carsCacheWarmer := cache.NewNoopCarsCacheWarmer()
	service := service.NewService(repo, carsCache, carsCacheWarmer)

	handler := handler.NewHandler(service, &trustedKey.PublicKey)
	return routes(handler), db
}

func executeRequest(
	t *testing.T,
	app http.Handler,
	method, path, token string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("failed to encode body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	app.ServeHTTP(rec, req)

	return rec
}

func strPtr(s string) *string {
	return &s
}

func int64Ptr(i int64) *int64 {
	return &i
}

func seedCars(t *testing.T, db *gorm.DB, cars []domain.Car) []domain.Car {
	t.Helper()

	seededCars := make([]domain.Car, len(cars))
	for i, car := range cars {
		if result := db.Create(&car); result.Error != nil {
			t.Fatal(result.Error)
		}
		seededCars[i] = car
	}

	return seededCars
}
