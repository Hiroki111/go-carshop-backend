package serviceauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClient(t *testing.T, handler http.HandlerFunc, mutate ...func(*Config)) (*ClientCredentials, *fakeClock) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	cfg := Config{
		TokenURL:     srv.URL + "/oauth/token",
		ClientID:     "order-service",
		ClientSecret: "secret",
		Scopes:       []string{"cars:reserve"},
	}
	for _, m := range mutate {
		m(&cfg)
	}

	c, err := NewClientCredentials(cfg)
	require.NoError(t, err)

	clock := &fakeClock{t: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)}
	c.now = clock.Now

	return c, clock
}

func tokenHandler(expiresIn int, calls *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("token-%d", n),
			"token_type":   "Bearer",
			"expires_in":   expiresIn,
			"scope":        "cars:reserve",
		})
	}
}

func TestToken_IsFetchedOnceAndReused(t *testing.T) {
	var calls atomic.Int32
	c, clock := newClient(t, tokenHandler(300, &calls))

	for i := 0; i < 3; i++ {
		token, err := c.Token(context.Background())
		require.NoError(t, err)
		require.Equal(t, "token-1", token)
		clock.Advance(time.Minute)
	}

	require.EqualValues(t, 1, calls.Load())
}

func TestToken_IsReplacedShortlyBeforeItExpires(t *testing.T) {
	var calls atomic.Int32
	c, clock := newClient(t, tokenHandler(300, &calls)) // lives 5 minutes

	first, err := c.Token(context.Background())
	require.NoError(t, err)

	// 4 minutes in, still comfortably valid.
	clock.Advance(4 * time.Minute)
	token, err := c.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, first, token)
	require.EqualValues(t, 1, calls.Load())

	// 4m45s in: the token is valid for another 15 seconds, which is inside the
	// 30 second margin, so it is replaced.
	clock.Advance(45 * time.Second)
	token, err = c.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-2", token)
	require.EqualValues(t, 2, calls.Load())
}

func TestToken_ShortLivedTokenIsNotRefetchedOnEveryCall(t *testing.T) {
	var calls atomic.Int32
	c, _ := newClient(t, tokenHandler(10, &calls)) // shorter than the margin

	for i := 0; i < 3; i++ {
		_, err := c.Token(context.Background())
		require.NoError(t, err)
	}

	require.EqualValues(t, 1, calls.Load())
}

func TestToken_ConcurrentCallersShareOneFetch(t *testing.T) {
	var calls atomic.Int32
	c, _ := newClient(t, tokenHandler(300, &calls))

	const callers = 20
	tokens := make([]string, callers)
	errs := make([]error, callers)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			tokens[i], errs[i] = c.Token(context.Background())
		}(i)
	}
	close(start)
	wg.Wait()

	for i := 0; i < callers; i++ {
		require.NoError(t, errs[i])
		require.Equal(t, "token-1", tokens[i])
	}
	require.EqualValues(t, 1, calls.Load())
}

func TestToken_RequestFollowsTheClientCredentialsGrant(t *testing.T) {
	// A secret with characters that must be form-encoded before the Basic
	// header is built (RFC 6749 section 2.3.1).
	const clientID, clientSecret = "order-service", "p+ss:w%rd /1"

	type seenRequest struct {
		method, contentType, grantType, scope, id, secret string
		basicOK                                           bool
	}
	seen := make(chan seenRequest, 1)

	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		request := seenRequest{
			method:      r.Method,
			contentType: r.Header.Get("Content-Type"),
			grantType:   r.PostForm.Get("grant_type"),
			scope:       r.PostForm.Get("scope"),
		}

		id, secret, ok := r.BasicAuth()
		request.basicOK = ok
		var err error
		request.id, err = url.QueryUnescape(id)
		assert.NoError(t, err)
		request.secret, err = url.QueryUnescape(secret)
		assert.NoError(t, err)
		seen <- request

		var calls atomic.Int32
		tokenHandler(300, &calls)(w, r)
	}

	c, _ := newClient(t, handler, func(cfg *Config) {
		cfg.ClientID = clientID
		cfg.ClientSecret = clientSecret
		cfg.Scopes = []string{"cars:reserve", "cars:release"}
	})

	_, err := c.Token(context.Background())
	require.NoError(t, err)

	got := <-seen
	require.Equal(t, http.MethodPost, got.method)
	require.Equal(t, "application/x-www-form-urlencoded", got.contentType)
	require.Equal(t, "client_credentials", got.grantType)
	require.Equal(t, "cars:reserve cars:release", got.scope)
	require.True(t, got.basicOK, "the client must authenticate with HTTP Basic")
	require.Equal(t, clientID, got.id)
	require.Equal(t, clientSecret, got.secret)
}

func TestToken_OmitsScopeWhenNoneIsConfigured(t *testing.T) {
	scopeSent := make(chan bool, 1)
	handler := func(w http.ResponseWriter, r *http.Request) {
		assert.NoError(t, r.ParseForm())
		_, ok := r.PostForm["scope"]
		scopeSent <- ok

		var calls atomic.Int32
		tokenHandler(300, &calls)(w, r)
	}
	c, _ := newClient(t, handler, func(cfg *Config) { cfg.Scopes = nil })

	_, err := c.Token(context.Background())
	require.NoError(t, err)
	require.False(t, <-scopeSent)
}

func TestToken_ErrorsAreReportedAndNotCached(t *testing.T) {
	var calls atomic.Int32
	var failing atomic.Bool
	failing.Store(true)

	handler := func(w http.ResponseWriter, r *http.Request) {
		if failing.Load() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"client authentication failed"}`))
			calls.Add(1)
			return
		}
		tokenHandler(300, &calls)(w, r)
	}
	c, _ := newClient(t, handler)

	_, err := c.Token(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
	require.Contains(t, err.Error(), "invalid_client")
	require.NotContains(t, err.Error(), "secret", "the error must not leak the secret")

	// Once user-service recovers, the next call just works.
	failing.Store(false)
	token, err := c.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, "token-2", token)
}

func TestToken_FailedRefreshFallsBackToTheTokenThatIsStillValid(t *testing.T) {
	var calls atomic.Int32
	var down atomic.Bool

	handler := func(w http.ResponseWriter, r *http.Request) {
		if down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		tokenHandler(300, &calls)(w, r)
	}
	c, clock := newClient(t, handler)

	first, err := c.Token(context.Background())
	require.NoError(t, err)

	// Inside the refresh margin, with user-service down: the old token still
	// has 15 seconds left, so it is used.
	clock.Advance(4*time.Minute + 45*time.Second)
	down.Store(true)
	token, err := c.Token(context.Background())
	require.NoError(t, err)
	require.Equal(t, first, token)

	// Once it has really expired there is nothing to fall back to.
	clock.Advance(time.Minute)
	_, err = c.Token(context.Background())
	require.Error(t, err)
}

func TestToken_RejectsMalformedTokenResponses(t *testing.T) {
	tests := map[string]string{
		"not JSON":              `not json`,
		"no access token":       `{"token_type":"Bearer","expires_in":300}`,
		"not a bearer token":    `{"access_token":"x","token_type":"mac","expires_in":300}`,
		"no lifetime":           `{"access_token":"x","token_type":"Bearer"}`,
		"negative lifetime":     `{"access_token":"x","token_type":"Bearer","expires_in":-5}`,
		"empty response object": `{}`,
	}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			})

			token, err := c.Token(context.Background())
			require.Error(t, err)
			require.Empty(t, token)
		})
	}
}

func TestToken_CallerContextCancelsTheFetch(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	c, _ := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Token(ctx)
	require.ErrorIs(t, err, context.Canceled)
}

func TestNewClientCredentials_RequiresItsSettings(t *testing.T) {
	valid := Config{TokenURL: "http://user-service/oauth/token", ClientID: "order-service", ClientSecret: "secret"}

	tests := map[string]func(*Config){
		"no token URL":     func(c *Config) { c.TokenURL = "" },
		"no client ID":     func(c *Config) { c.ClientID = "" },
		"no client secret": func(c *Config) { c.ClientSecret = "" },
	}

	_, err := NewClientCredentials(valid)
	require.NoError(t, err)

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := valid
			mutate(&cfg)
			_, err := NewClientCredentials(cfg)
			require.Error(t, err)
		})
	}
}
