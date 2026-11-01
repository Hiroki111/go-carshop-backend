package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/domain"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/service"
)

// RegisterCustomer godoc
// @Summary      Register a Customer
// @Description  Creates a new user with the Customer role.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        user  body      RegisterUserRequest  true  "Customer Registration Payload"
// @Success      201   {object}  RegisterUserResponse
// @Failure      400   {object}  ErrorResponse
// @Failure      409   {object}  ErrorResponse
// @Failure      500   {object}  ErrorResponse
// @Router       /register-customer [post]
func (h *Handler) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	var data RegisterUserRequest

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validate.Struct(data); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: h.formatValidationError(err),
		})
		return
	}

	err := h.service.CreateUser(domain.User{
		UserName: data.UserName,
		Password: data.Password,
		Email:    data.Email,
		Role:     domain.CustomerRole,
	})
	if err != nil {
		if errors.Is(err, repository.ErrUserAlreadyExists) {
			writeJSON(w, http.StatusConflict, ErrorResponse{
				Error: "user already exists",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to create user",
		})
		return
	}

	writeJSON(w, http.StatusCreated, RegisterUserResponse{
		Status: "user created",
	})
}

// LoginUser godoc
// @Summary      User Login
// @Description  Authenticates a user and returns a JWT access token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        login  body      LoginUserRequest  true  "Login Credentials"
// @Success      200    {object}  LoginUserResponse
// @Failure      400    {object}  ErrorResponse
// @Failure      401    {object}  ErrorResponse
// @Failure      500    {object}  ErrorResponse
// @Router       /login-user [post]
func (h *Handler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var data LoginUserRequest

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid request body",
		})
		return
	}

	if err := h.validate.Struct(data); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: h.formatValidationError(err),
		})
		return
	}

	token, err := h.service.Login(data.UserName, data.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{
				Error: "invalid username or password",
			})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "failed to log in",
		})
		return
	}

	writeJSON(w, http.StatusOK, LoginUserResponse{
		AccessToken: token,
		TokenType:   "Bearer",
	})
}

// GetInternalUser godoc
// @Summary      Look a user up (service to service)
// @Description  Returns the ID, user name and email of a user. For other services only: the caller needs a service token with the users:read scope (see POST /oauth/token). The role and the password hash are never returned.
// @Tags         internal
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      int  true  "User ID"
// @Success      200  {object}  GetInternalUserResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /internal/users/{id} [get]
func (h *Handler) GetInternalUser(w http.ResponseWriter, r *http.Request) {
	id64, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id64 <= 0 {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "invalid ID",
		})
		return
	}

	user, err := h.service.GetUserByID(uint(id64))
	if err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{
				Error: "user not found",
			})
			return
		}

		log.Printf("user %d: failed to look the user up: %v", id64, err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error: "internal error",
		})
		return
	}

	writeJSON(w, http.StatusOK, GetInternalUserResponse{
		Item: InternalUserItem{
			ID:       user.ID,
			UserName: user.UserName,
			Email:    user.Email,
		},
	})
}
