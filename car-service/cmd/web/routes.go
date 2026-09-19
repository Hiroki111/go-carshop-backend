package main

import (
	"net/http"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/go-chi/chi/v5"
)

func routes(handler *handler.Handler) http.Handler {
	mux := chi.NewRouter()
	mux.Get("/ping", handler.Ping)

	return mux
}
