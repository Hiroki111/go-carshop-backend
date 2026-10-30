package handler

type ErrorResponse struct {
	Error string `json:"error"`
}

type RegisterUserRequest struct {
	UserName string `json:"user_name" validate:"required"`
	Email    string `json:"email" validate:"required,email,max=254"`
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

// TokenResponse is the successful reply of POST /oauth/token (RFC 6749 section 5.1).
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type" example:"Bearer"`
	// ExpiresIn is the lifetime of the token in seconds.
	ExpiresIn int `json:"expires_in" example:"300"`
	// Scope is the space-separated list of granted scopes.
	Scope string `json:"scope" example:"cars:reserve"`
}

// OAuthErrorResponse is the error reply of POST /oauth/token (RFC 6749 section 5.2).
type OAuthErrorResponse struct {
	Error            string `json:"error" example:"invalid_client"`
	ErrorDescription string `json:"error_description,omitempty"`
}
