package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func newKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	return key
}

func sign(t *testing.T, method jwt.SigningMethod, key *rsa.PrivateKey, claims Claims) string {
	t.Helper()

	s, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func validClaims(role UserRole) Claims {
	return Claims{
		UserID: 1,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	}
}

func TestParseJWTToken_ValidToken(t *testing.T) {
	key := newKey(t)
	token := sign(t, jwt.SigningMethodRS256, key, validClaims(AdminRole))

	identity, err := ParseJWTToken(&key.PublicKey, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := (Identity{UserID: 1, Role: AdminRole}); identity != want {
		t.Fatalf("expected %+v, got %+v", want, identity)
	}
}

func TestParseJWTToken_ServiceToken(t *testing.T) {
	key := newKey(t)

	claims := validClaims("")
	claims.UserID = 0
	claims.Subject = "order-service"
	claims.Scope = "cars:reserve cars:release"
	token := sign(t, jwt.SigningMethodRS256, key, claims)

	identity, err := ParseJWTToken(&key.PublicKey, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := Identity{Subject: "order-service", Scope: "cars:reserve cars:release"}
	if identity != want {
		t.Fatalf("expected %+v, got %+v", want, identity)
	}
}

func TestHasScope(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		required string
		want     bool
	}{
		{name: "empty scope", scope: "", required: ScopeCarsReserve, want: false},
		{name: "exact match", scope: "cars:reserve", required: ScopeCarsReserve, want: true},
		{name: "one of several", scope: "cars:release cars:reserve", required: ScopeCarsReserve, want: true},
		{name: "surrounding whitespace", scope: "  cars:reserve  ", required: ScopeCarsReserve, want: true},
		{name: "longer scope with the same prefix", scope: "cars:reserved", required: ScopeCarsReserve, want: false},
		{name: "shorter scope", scope: "cars", required: ScopeCarsReserve, want: false},
		{name: "different scope", scope: "cars:release", required: ScopeCarsReserve, want: false},
		{name: "empty required scope is never granted", scope: "cars:reserve", required: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := HasScope(test.scope, test.required); got != test.want {
				t.Fatalf("HasScope(%q, %q) = %v, want %v", test.scope, test.required, got, test.want)
			}
		})
	}
}

func TestParseJWTToken_Rejections(t *testing.T) {
	key := newKey(t)
	otherKey := newKey(t)

	expired := validClaims(AdminRole)
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

	noExpiry := validClaims(AdminRole)
	noExpiry.ExpiresAt = nil

	tests := []struct {
		name      string
		publicKey *rsa.PublicKey
		token     string
		wantErr   error // nil means "any error"
	}{
		{name: "malformed token", publicKey: &key.PublicKey, token: "this.is.not.a.jwt"},
		{name: "expired token", publicKey: &key.PublicKey, token: sign(t, jwt.SigningMethodRS256, key, expired), wantErr: jwt.ErrTokenExpired},
		{name: "missing expiry", publicKey: &key.PublicKey, token: sign(t, jwt.SigningMethodRS256, key, noExpiry)},
		{name: "signed by another key", publicKey: &key.PublicKey, token: sign(t, jwt.SigningMethodRS256, otherKey, validClaims(AdminRole)), wantErr: jwt.ErrTokenSignatureInvalid},
		{name: "other RSA algorithm", publicKey: &key.PublicKey, token: sign(t, jwt.SigningMethodRS512, key, validClaims(AdminRole))},
		{name: "nil public key", publicKey: nil, token: sign(t, jwt.SigningMethodRS256, key, validClaims(AdminRole))},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseJWTToken(test.publicKey, test.token)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("expected error %v, got %v", test.wantErr, err)
			}
		})
	}
}
