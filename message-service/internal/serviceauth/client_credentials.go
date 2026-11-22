package serviceauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	refreshMargin = 30 * time.Second

	maxResponseBytes = 1 << 20
)

type Config struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	Scopes       []string
	HTTPClient   *http.Client
}

type ClientCredentials struct {
	tokenURL     string
	clientID     string
	clientSecret string
	scope        string
	httpClient   *http.Client
	now          func() time.Time

	mu        sync.Mutex
	token     string
	refreshAt time.Time
	expiresAt time.Time
}

func NewClientCredentials(cfg Config) (*ClientCredentials, error) {
	switch {
	case cfg.TokenURL == "":
		return nil, errors.New("serviceauth: token URL is required")
	case cfg.ClientID == "":
		return nil, errors.New("serviceauth: client ID is required")
	case cfg.ClientSecret == "":
		return nil, errors.New("serviceauth: client secret is required")
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}

	return &ClientCredentials{
		tokenURL:     cfg.TokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		scope:        strings.Join(cfg.Scopes, " "),
		httpClient:   httpClient,
		now:          time.Now,
	}, nil
}

func (c *ClientCredentials) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if c.token != "" && now.Before(c.refreshAt) {
		return c.token, nil
	}

	fresh, err := c.fetch(ctx)
	if err != nil {
		if c.token != "" && now.Before(c.expiresAt) {
			return c.token, nil
		}
		return "", err
	}

	ttl := time.Duration(fresh.ExpiresIn) * time.Second
	refreshIn := ttl - refreshMargin
	if refreshIn <= 0 {
		refreshIn = ttl / 2
	}

	c.token = fresh.AccessToken
	c.refreshAt = now.Add(refreshIn)
	c.expiresAt = now.Add(ttl)

	return c.token, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type tokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func (c *ClientCredentials) fetch(ctx context.Context) (tokenResponse, error) {
	form := url.Values{"grant_type": {"client_credentials"}}
	if c.scope != "" {
		form.Set("scope", c.scope)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, fmt.Errorf("serviceauth: failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	// RFC 6749 section 2.3.1: the ID and secret are form-urlencoded before they
	// are Base64-encoded into the Basic header.
	req.SetBasicAuth(url.QueryEscape(c.clientID), url.QueryEscape(c.clientSecret))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("serviceauth: failed to reach the token endpoint: %w", err)
	}
	defer resp.Body.Close()

	body := io.LimitReader(resp.Body, maxResponseBytes)

	if resp.StatusCode != http.StatusOK {
		var oauthErr tokenErrorResponse
		_ = json.NewDecoder(body).Decode(&oauthErr)
		if oauthErr.Error != "" {
			return tokenResponse{}, fmt.Errorf("serviceauth: token endpoint returned %d: %s (%s)", resp.StatusCode, oauthErr.Error, oauthErr.ErrorDescription)
		}
		return tokenResponse{}, fmt.Errorf("serviceauth: token endpoint returned %d", resp.StatusCode)
	}

	var tr tokenResponse
	if err := json.NewDecoder(body).Decode(&tr); err != nil {
		return tokenResponse{}, fmt.Errorf("serviceauth: failed to decode the token response: %w", err)
	}
	if tr.AccessToken == "" {
		return tokenResponse{}, errors.New("serviceauth: token response has no access_token")
	}
	if !strings.EqualFold(tr.TokenType, "Bearer") {
		return tokenResponse{}, fmt.Errorf("serviceauth: unsupported token_type %q", tr.TokenType)
	}
	if tr.ExpiresIn <= 0 {
		return tokenResponse{}, fmt.Errorf("serviceauth: token response has an invalid expires_in %d", tr.ExpiresIn)
	}

	return tr, nil
}
