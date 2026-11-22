package userclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type HTTPUserClient struct {
	baseURL    string
	httpClient *http.Client
	tokens     TokenSource
}

func NewHTTPUserClient(baseURL string, tokens TokenSource) *HTTPUserClient {
	return &HTTPUserClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		tokens: tokens,
	}
}

func (c *HTTPUserClient) GetUser(ctx context.Context, id uint) (User, error) {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return User{}, fmt.Errorf("userclient: failed to get a service token: %w", err)
	}

	url := fmt.Sprintf("%s/internal/users/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return User{}, fmt.Errorf("userclient: failed to build request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return User{}, fmt.Errorf("userclient: failed to reach user-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return User{}, ErrUserNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return User{}, fmt.Errorf("userclient: unexpected status code %d", resp.StatusCode)
	}

	return decodeUser(resp)
}

type internalUserItem struct {
	ID       uint   `json:"id"`
	UserName string `json:"user_name"`
	Email    string `json:"email"`
}

type getUserResponse struct {
	Item internalUserItem `json:"item"`
}

func decodeUser(resp *http.Response) (User, error) {
	var body getUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return User{}, fmt.Errorf("userclient: failed to decode response: %w", err)
	}

	if body.Item.Email == "" {
		return User{}, errors.New("userclient: response has no email")
	}

	return User{
		ID:       body.Item.ID,
		UserName: body.Item.UserName,
		Email:    body.Item.Email,
	}, nil
}
