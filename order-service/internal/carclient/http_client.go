package carclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPCarClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewHTTPCarClient(baseURL string) *HTTPCarClient {
	return &HTTPCarClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type carItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	PriceCents  uint   `json:"price_cents"`
	IsAvailable bool   `json:"is_available"`
}

type getCarResponse struct {
	Item carItem `json:"item"`
}

func (c *HTTPCarClient) GetCarByID(ctx context.Context, id uint) (Car, error) {
	url := fmt.Sprintf("%s/cars/%d", c.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Car{}, fmt.Errorf("carclient: failed to build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Car{}, fmt.Errorf("carclient: failed to reach car-service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Car{}, ErrCarNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return Car{}, fmt.Errorf("carclient: unexpected status code %d", resp.StatusCode)
	}

	var body getCarResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Car{}, fmt.Errorf("carclient: failed to decode response: %w", err)
	}

	return Car{
		ID:          body.Item.ID,
		Name:        body.Item.Name,
		PriceCents:  body.Item.PriceCents,
		IsAvailable: body.Item.IsAvailable,
	}, nil
}
