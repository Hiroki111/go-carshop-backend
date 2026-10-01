package handler

type ErrorResponse struct {
	Error string `json:"error"`
}

type CreateOrderRequest struct {
	CarID uint `json:"car_id"`
}

type CreateOrderResponse struct {
	Message string `json:"message"`
}

// TODO: CustomerName should be added once a user service exists to resolve
// a UserID into a display name.
type OrderItem struct {
	ID uint `json:"id"`
	// CustomerName string `json:"customer_name"`
	CarName    string `json:"car_name"`
	PriceCents uint   `json:"price_cents"`
}

type GetOrdersResponse struct {
	Items   []OrderItem `json:"items"`
	Page    int         `json:"page"`
	Limit   int         `json:"limit"`
	Total   int         `json:"total"`
	HasNext bool        `json:"hasNext"`
}

type GetOrderResponse struct {
	Item OrderItem `json:"item"`
}

type UpdateOrderResponse struct {
	Item OrderItem `json:"item"`
}

type UpdateOrderRequest struct {
	PriceCents *uint   `json:"price_cents"`
	CarName    *string `json:"car_name"`
}

type DeleteOrderResponse struct {
	Message string `json:"message"`
}
