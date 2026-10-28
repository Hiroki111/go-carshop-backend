package carclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type HTTPCarClient struct {
	baseURL    string
	httpClient *http.Client
	tokens     TokenSource
}

func NewHTTPCarClient(baseURL string, tokens TokenSource) *HTTPCarClient {
	return &HTTPCarClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		tokens: tokens,
	}
}

type carItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PriceCents uint   `json:"price_cents"`
	Status     string `json:"status"`
}

type getCarResponse struct {
	Item carItem `json:"item"`
}

type orderCommandRequest struct {
	OrderID uint `json:"order_id"`
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

	return decodeCar(resp)
}

func (c *HTTPCarClient) ReserveCar(ctx context.Context, carID, orderID uint) (Car, error) {
	resp, err := c.postForOrder(ctx, fmt.Sprintf("/internal/cars/%d/reservations", carID), orderID)
	if err != nil {
		return Car{}, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return decodeCar(resp)
	case http.StatusNotFound:
		return Car{}, ErrCarNotFound
	case http.StatusConflict:
		return Car{}, ErrCarNotAvailable
	default:
		return Car{}, fmt.Errorf("carclient: reserving car %d for order %d: unexpected status code %d", carID, orderID, resp.StatusCode)
	}
}

func (c *HTTPCarClient) ReleaseCar(ctx context.Context, carID, orderID uint) error {
	resp, err := c.postForOrder(ctx, fmt.Sprintf("/internal/cars/%d/release", carID), orderID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return ErrCarNotFound
	case http.StatusConflict:
		return ErrCarNotAvailable
	default:
		return fmt.Errorf("carclient: releasing car %d for order %d: unexpected status code %d", carID, orderID, resp.StatusCode)
	}
}

// postForOrder sends the POST that car-service's internal endpoints expect: the
// service token as a bearer token, and the order ID in the body. The caller
// closes the response body.
func (c *HTTPCarClient) postForOrder(ctx context.Context, path string, orderID uint) (*http.Response, error) {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("carclient: failed to get a service token: %w", err)
	}

	payload, err := json.Marshal(orderCommandRequest{OrderID: orderID})
	if err != nil {
		return nil, fmt.Errorf("carclient: failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("carclient: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("carclient: failed to reach car-service: %w", err)
	}

	return resp, nil
}

func decodeCar(resp *http.Response) (Car, error) {
	var body getCarResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return Car{}, fmt.Errorf("carclient: failed to decode response: %w", err)
	}

	return Car{
		ID:         body.Item.ID,
		Name:       body.Item.Name,
		PriceCents: body.Item.PriceCents,
		Status:     body.Item.Status,
	}, nil
}
