package handler

import (
	"net/http"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
)

func (h *Handler) RequireScope(requiredScope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(r.Header.Get("Authorization"), " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		identity, err := auth.ParseServiceToken(h.publicKey, parts[1])
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, ErrorResponse{Error: "unauthorized"})
			return
		}

		if !auth.HasScope(identity.Scope, requiredScope) {
			writeJSON(w, http.StatusForbidden, ErrorResponse{Error: "forbidden"})
			return
		}

		next.ServeHTTP(w, r)
	}
}
