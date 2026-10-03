package handler

type ErrorResponse struct {
	Error string `json:"error"`
}

type RegisterUserRequest struct {
	UserName string `json:"user_name" validate:"required"`
	Password string `json:"password" validate:"required,min=8"`
}

type RegisterUserResponse struct {
	Status string `json:"status"`
}

type LoginUserRequest struct {
	UserName string `json:"user_name" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type LoginUserResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}
