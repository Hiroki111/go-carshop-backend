package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
)

type contextKey string

const (
	UserIDKey  contextKey = "userID"
	RoleKey    contextKey = "role"
	SubjectKey contextKey = "subject"
	ScopeKey   contextKey = "scope"
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

		identity, err := auth.ParseJWTToken(h.publicKey, parts[1])
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, identity.UserID)
		ctx = context.WithValue(ctx, RoleKey, identity.Role)
		ctx = context.WithValue(ctx, SubjectKey, identity.Subject)
		ctx = context.WithValue(ctx, ScopeKey, identity.Scope)

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

func (h *Handler) RequireScope(requiredScope string, next http.HandlerFunc) http.HandlerFunc {
	return h.RequireToken(func(w http.ResponseWriter, r *http.Request) {
		scope, _ := r.Context().Value(ScopeKey).(string)

		if !auth.HasScope(scope, requiredScope) {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
