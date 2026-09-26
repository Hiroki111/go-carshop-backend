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

// TODO: OrderItem is an object that contains an ordder's information, including the customer and car name.
// Find out how to get the customer and car name and add them to an OrderItem
type OrderItem struct {
	ID uint `json:"id"`
	// CustomerName string `json:"customer_name"`
	// CarName      string `json:"car_name"`
	PriceCents uint `json:"price_cents"`
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
	PriceCents *uint `json:"price_cents"`
}

type DeleteOrderResponse struct {
	Message string `json:"message"`
}
