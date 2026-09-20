package handler

type ErrorResponse struct {
	Error string `json:"error"`
}

type CarItem struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	PriceCents uint   `json:"price_cents"`
}

type GetCarsResponse struct {
	Items   []CarItem `json:"items"`
	Page    int           `json:"page"`
	Limit   int           `json:"limit"`
	Total   int           `json:"total"`
	HasNext bool          `json:"hasNext"`
}

type GetCarResponse struct {
	Item CarItem `json:"item"`
}

type CreateCarRequest struct {
	Name       string `json:"name" validate:"required,min=2,max=100" example:"Car Name"`
	PriceCents uint   `json:"price_cents" example:"12000"`
}

type CreateCarResponse struct {
	Item CarItem `json:"item"`
}

type UpdateCarRequest struct {
	Name       *string `json:"name" validate:"min=2,max=100" example:"Car Name"`
	PriceCents *uint   `json:"price_cents" example:"12000"`
}

type UpdateCarResponse struct {
	Item CarItem `json:"item"`
}
type DeleteCarResponse struct {
	Message string `json:"message"`
}
