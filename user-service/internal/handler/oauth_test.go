package handler

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/oauth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/service"
	"github.com/golang-jwt/jwt/v5"
)

const (
	orderServiceSecret = "0123456789abcdef0123456789abcdef0123456789abcdef"

	// A secret full of characters that have a meaning in a URL or a form.
	trickySecret = "p@ss+word%20/with=odd&chars? and spaces!!"
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

func newTokenHandler(t *testing.T) *Handler {
	t.Helper()

	orderService, err := oauth.NewClient(
		"order-service",
		orderServiceSecret,
		[]string{
			oauth.ScopeCarsReserve,
			oauth.ScopeCarsRelease,
			oauth.ScopeCarsSold,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	trickyService, err := oauth.NewClient("tricky-service", trickySecret, []string{oauth.ScopeCarsReserve})
	if err != nil {
		t.Fatal(err)
	}
	registry, err := oauth.NewRegistry(orderService, trickyService)
	if err != nil {
		t.Fatal(err)
	}

	return NewHandler(service.NewService(nil, testKey, registry))
}

// requestMod changes a token request before it is sent.
type requestMod func(r *http.Request)

func withBasicAuth(id, secret string) requestMod {
	return func(r *http.Request) { r.SetBasicAuth(id, secret) }
}

// withEncodedBasicAuth sends the credentials the way RFC 6749 section 2.3.1
// asks for: form-urlencoded before the usual Basic encoding.
func withEncodedBasicAuth(id, secret string) requestMod {
	return withBasicAuth(url.QueryEscape(id), url.QueryEscape(secret))
}

func withHeader(name, value string) requestMod {
	return func(r *http.Request) { r.Header.Set(name, value) }
}

func postToken(h *Handler, form url.Values, mods ...requestMod) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/oauth/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, mod := range mods {
		mod(req)
	}

	rec := httptest.NewRecorder()
	h.IssueToken(rec, req)
	return rec
}

func clientCredentialsForm(extra url.Values) url.Values {
	form := url.Values{"grant_type": {"client_credentials"}}
	for name, values := range extra {
		form[name] = values
	}
	return form
}

func TestIssueToken_Success(t *testing.T) {
	allScopes := "cars:reserve cars:release cars:sold"

	tests := []struct {
		testName      string
		form          url.Values
		mods          []requestMod
		expectedSub   string
		expectedScope string
	}{
		{
			testName:      "HTTP Basic, requested scope",
			form:          clientCredentialsForm(url.Values{"scope": {"cars:reserve"}}),
			mods:          []requestMod{withBasicAuth("order-service", orderServiceSecret)},
			expectedSub:   "order-service",
			expectedScope: "cars:reserve",
		},
		{
			testName:      "HTTP Basic, no scope means everything the client is allowed",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withBasicAuth("order-service", orderServiceSecret)},
			expectedSub:   "order-service",
			expectedScope: allScopes,
		},
		{
			testName:      "credentials in the body",
			form:          clientCredentialsForm(url.Values{"client_id": {"order-service"}, "client_secret": {orderServiceSecret}, "scope": {"cars:release cars:sold"}}),
			expectedSub:   "order-service",
			expectedScope: "cars:release cars:sold",
		},
		{
			// RFC 6749 section 2.3.1: Basic credentials are form-urlencoded first.
			testName:      "HTTP Basic with form-urlencoded credentials",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withEncodedBasicAuth("tricky-service", trickySecret)},
			expectedSub:   "tricky-service",
			expectedScope: "cars:reserve",
		},
		{
			testName:      "credentials in the body with odd characters in the secret",
			form:          clientCredentialsForm(url.Values{"client_id": {"tricky-service"}, "client_secret": {trickySecret}}),
			expectedSub:   "tricky-service",
			expectedScope: "cars:reserve",
		},
		{
			testName:      "Basic scheme name is case-insensitive",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withBasicAuth("order-service", orderServiceSecret), lowercaseAuthScheme},
			expectedSub:   "order-service",
			expectedScope: allScopes,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			h := newTokenHandler(t)

			before := time.Now().Add(-time.Second)
			rec := postToken(h, test.form, test.mods...)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected code %d, got %d (body: %s)", http.StatusOK, rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("expected Cache-Control no-store, got %q", got)
			}
			if got := rec.Header().Get("Pragma"); got != "no-cache" {
				t.Errorf("expected Pragma no-cache, got %q", got)
			}

			var resp TokenResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.TokenType != "Bearer" {
				t.Errorf("expected token_type Bearer, got %q", resp.TokenType)
			}
			if want := int(auth.ServiceTokenTTL.Seconds()); resp.ExpiresIn != want {
				t.Errorf("expected expires_in %d, got %d", want, resp.ExpiresIn)
			}
			if resp.Scope != test.expectedScope {
				t.Errorf("expected scope %q, got %q", test.expectedScope, resp.Scope)
			}

			// The token is signed with our key and says what the response says.
			claims := &auth.ServiceClaims{}
			token, err := jwt.ParseWithClaims(
				resp.AccessToken, claims,
				func(*jwt.Token) (interface{}, error) { return &testKey.PublicKey, nil },
				jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
				jwt.WithExpirationRequired(),
			)
			if err != nil || !token.Valid {
				t.Fatalf("the access token is not valid: %v", err)
			}
			if claims.Subject != test.expectedSub {
				t.Errorf("expected subject %q, got %q", test.expectedSub, claims.Subject)
			}
			if claims.Scope != test.expectedScope {
				t.Errorf("expected scope claim %q, got %q", test.expectedScope, claims.Scope)
			}
			if exp := claims.ExpiresAt.Time; exp.Before(before.Add(auth.ServiceTokenTTL)) || exp.After(time.Now().Add(auth.ServiceTokenTTL+time.Second)) {
				t.Errorf("unexpected expiry %v", exp)
			}
		})
	}
}

func lowercaseAuthScheme(r *http.Request) {
	header := r.Header.Get("Authorization")
	r.Header.Set("Authorization", strings.ToLower(header[:len("Basic")])+header[len("Basic"):])
}

func TestIssueToken_Rejections(t *testing.T) {
	validBasic := withBasicAuth("order-service", orderServiceSecret)

	tests := []struct {
		testName      string
		form          url.Values
		mods          []requestMod
		query         string // appended to the URL
		expectedCode  int
		expectedError string
	}{
		// Who is asking?
		{
			testName:      "wrong secret",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withBasicAuth("order-service", orderServiceSecret+"x")},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "unknown client",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withBasicAuth("nobody", orderServiceSecret)},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "no credentials at all",
			form:          clientCredentialsForm(nil),
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "empty secret in the body",
			form:          clientCredentialsForm(url.Values{"client_id": {"order-service"}, "client_secret": {""}}),
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "malformed Basic header",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withHeader("Authorization", "Basic !!!not-base64!!!")},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "a Bearer token is not client authentication",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{withHeader("Authorization", "Bearer abc.def.ghi")},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			// Credentials in the URL would end up in logs, so they are ignored.
			testName:      "credentials only in the query string",
			form:          clientCredentialsForm(nil),
			query:         "?client_id=order-service&client_secret=" + orderServiceSecret,
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},
		{
			testName:      "two authentication methods at once",
			form:          clientCredentialsForm(url.Values{"client_id": {"order-service"}, "client_secret": {orderServiceSecret}}),
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},

		// What are they asking for?
		{
			testName:      "grant_type missing",
			form:          url.Values{},
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},
		{
			testName:      "grant_type repeated",
			form:          url.Values{"grant_type": {"client_credentials", "client_credentials"}},
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},
		{
			testName:      "unsupported grant type",
			form:          url.Values{"grant_type": {"password"}, "username": {"admin"}, "password": {"secret"}},
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "unsupported_grant_type",
		},
		{
			testName:      "scope the client may not have",
			form:          clientCredentialsForm(url.Values{"scope": {"cars:delete"}}),
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_scope",
		},
		{
			testName:      "one good and one bad scope gets nothing",
			form:          clientCredentialsForm(url.Values{"scope": {"cars:reserve cars:delete"}}),
			mods:          []requestMod{validBasic},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_scope",
		},
		{
			testName:      "scope that another client has",
			form:          clientCredentialsForm(url.Values{"scope": {"cars:release"}}),
			mods:          []requestMod{withEncodedBasicAuth("tricky-service", trickySecret)},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_scope",
		},
		{
			testName:      "wrong secret is reported before the scope is looked at",
			form:          clientCredentialsForm(url.Values{"scope": {"cars:delete"}}),
			mods:          []requestMod{withBasicAuth("order-service", "wrong")},
			expectedCode:  http.StatusUnauthorized,
			expectedError: "invalid_client",
		},

		// How are they asking?
		{
			testName:      "JSON body",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{validBasic, withHeader("Content-Type", "application/json")},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},
		{
			testName:      "no Content-Type",
			form:          clientCredentialsForm(nil),
			mods:          []requestMod{validBasic, withHeader("Content-Type", "")},
			expectedCode:  http.StatusBadRequest,
			expectedError: "invalid_request",
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			h := newTokenHandler(t)

			req := httptest.NewRequest(http.MethodPost, "/oauth/token"+test.query, strings.NewReader(test.form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for _, mod := range test.mods {
				mod(req)
			}
			rec := httptest.NewRecorder()
			h.IssueToken(rec, req)

			if rec.Code != test.expectedCode {
				t.Fatalf("expected code %d, got %d (body: %s)", test.expectedCode, rec.Code, rec.Body.String())
			}

			var resp OAuthErrorResponse
			if err := json.NewDecoder(strings.NewReader(rec.Body.String())).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Error != test.expectedError {
				t.Errorf("expected error %q, got %q", test.expectedError, resp.Error)
			}
			if strings.Contains(rec.Body.String(), "access_token") {
				t.Errorf("a rejected request must not get a token, got: %s", rec.Body.String())
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("expected Cache-Control no-store, got %q", got)
			}
			if test.expectedCode == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("a 401 must carry a WWW-Authenticate header")
			}
		})
	}
}

func TestIssueToken_RejectsAnOversizedBody(t *testing.T) {
	h := newTokenHandler(t)

	form := clientCredentialsForm(url.Values{"scope": {strings.Repeat("a", maxTokenRequestBytes)}})
	rec := postToken(h, form, withBasicAuth("order-service", orderServiceSecret))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected code %d, got %d (body: %s)", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}
