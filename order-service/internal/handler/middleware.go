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
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		userId, role, err := auth.ParseJWTToken(parts[1])
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		if role != requiredRole {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
