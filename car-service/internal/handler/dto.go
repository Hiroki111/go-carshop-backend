package handler

type ErrorResponse struct {
	Error string `json:"error"`
}

type ProductItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PriceCents uint   `json:"price_cents"`
}

type GetProductsResponse struct {
	Items   []ProductItem `json:"items"`
	Page    int           `json:"page"`
	Limit   int           `json:"limit"`
	Total   int           `json:"total"`
	HasNext bool          `json:"hasNext"`
}

type GetProductResponse struct {
	Item ProductItem `json:"item"`
}

type CreateProductRequest struct {
	Name       string `json:"name" validate:"required,min=2,max=100" example:"Product Name"`
	PriceCents uint   `json:"price_cents" example:"12000"`
}

type CreateProductResponse struct {
	Item ProductItem `json:"item"`
}

type UpdateProductRequest struct {
	Name       *string `json:"name" validate:"min=2,max=100" example:"Product Name"`
	PriceCents *uint   `json:"price_cents" example:"12000"`
}

type UpdateProductResponse struct {
	Item ProductItem `json:"item"`
}
type DeleteProductResponse struct {
	Message string `json:"message"`
}
