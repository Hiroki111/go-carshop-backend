package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var testKey = mustGenerateKey()

func mustGenerateKey() *rsa.PrivateKey {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return key
}

type carServiceClaims struct {
	UserID uint   `json:"user_id"`
	Role   string `json:"role"`
	Scope  string `json:"scope"`
	jwt.RegisteredClaims
}

func parse(t *testing.T, tokenString string, claims jwt.Claims) *jwt.Token {
	t.Helper()

	token, err := jwt.ParseWithClaims(
		tokenString, claims,
		func(*jwt.Token) (interface{}, error) { return &testKey.PublicKey, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		t.Fatalf("failed to parse the token: %v", err)
	}
	return token
}

func TestGenerateServiceToken(t *testing.T) {
	before := time.Now().Add(-time.Second)

	tokenString, err := GenerateServiceToken(testKey, "order-service", []string{"cars:reserve", "cars:release"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims := &ServiceClaims{}
	token := parse(t, tokenString, claims)

	if token.Method.Alg() != "RS256" {
		t.Errorf("expected RS256, got %s", token.Method.Alg())
	}
	if claims.Subject != "order-service" {
		t.Errorf("expected subject %q, got %q", "order-service", claims.Subject)
	}
	if claims.Issuer != serviceTokenIssuer {
		t.Errorf("expected issuer %q, got %q", serviceTokenIssuer, claims.Issuer)
	}
	if claims.Scope != "cars:reserve cars:release" {
		t.Errorf("expected scope %q, got %q", "cars:reserve cars:release", claims.Scope)
	}
	if claims.ID == "" {
		t.Error("expected a token ID (jti)")
	}
	if iat := claims.IssuedAt.Time; iat.Before(before) || iat.After(time.Now().Add(time.Second)) {
		t.Errorf("unexpected issued-at %v", iat)
	}
	if got := claims.ExpiresAt.Sub(claims.IssuedAt.Time); got != ServiceTokenTTL {
		t.Errorf("expected a lifetime of %v, got %v", ServiceTokenTTL, got)
	}
}

func TestGenerateServiceToken_EveryTokenHasItsOwnID(t *testing.T) {
	first, err := GenerateServiceToken(testKey, "order-service", []string{"cars:reserve"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := GenerateServiceToken(testKey, "order-service", []string{"cars:reserve"})
	if err != nil {
		t.Fatal(err)
	}

	firstClaims, secondClaims := &ServiceClaims{}, &ServiceClaims{}
	parse(t, first, firstClaims)
	parse(t, second, secondClaims)

	if firstClaims.ID == secondClaims.ID {
		t.Fatalf("expected different token IDs, both are %q", firstClaims.ID)
	}
}

func TestGenerateServiceToken_IsNotAUserToken(t *testing.T) {
	tokenString, err := GenerateServiceToken(testKey, "order-service", []string{"cars:reserve"})
	if err != nil {
		t.Fatal(err)
	}

	claims := &carServiceClaims{}
	parse(t, tokenString, claims)
	if claims.UserID != 0 || claims.Role != "" {
		t.Errorf("expected no user ID and no role, got user ID %d and role %q", claims.UserID, claims.Role)
	}
	if claims.Scope != "cars:reserve" {
		t.Errorf("expected scope %q, got %q", "cars:reserve", claims.Scope)
	}

	payload := strings.Split(tokenString, ".")[1]
	for _, field := range []string{"user_id", "user_name", "role"} {
		if strings.Contains(decodeSegment(t, payload), `"`+field+`"`) {
			t.Errorf("the token payload must not contain %q", field)
		}
	}
}

func decodeSegment(t *testing.T, segment string) string {
	t.Helper()

	decoded, err := base64.RawURLEncoding.DecodeString(segment)
	if err != nil {
		t.Fatalf("failed to decode a token segment: %v", err)
	}
	return string(decoded)
}
