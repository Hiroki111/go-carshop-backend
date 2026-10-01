package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
)

type contextKey string

const (
	UserIDKey contextKey = "userID"
	RoleKey   contextKey = "role"
)

func (h *Handler) RequireToken(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		userId, role, err := auth.ParseJWTToken(parts[1])
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userId)
		ctx = context.WithValue(ctx, RoleKey, role)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

func (h *Handler) RequireRole(requiredRole auth.UserRole, next http.HandlerFunc) http.HandlerFunc {
	return h.RequireToken(func(w http.ResponseWriter, r *http.Request) {
		role, ok := r.Context().Value(RoleKey).(auth.UserRole)
		if !ok {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "internal error"})
			return
		}

		if role != requiredRole {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
