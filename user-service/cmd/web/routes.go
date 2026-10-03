package main

import (
	"net/http"

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	_ "github.com/Hiroki111/go-carshop-backend/user-service/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

func routes(handler *handler.Handler) http.Handler {
	mux := chi.NewRouter()

	mux.Route("/", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		r.Post("/register-customer", handler.RegisterCustomer)
		r.Post("/login-user", handler.LoginUser)
	})

	// infra / public routes
	mux.Get("/ping", handler.Ping)

	// Swagger route
	mux.Get("/swagger/*", httpSwagger.WrapHandler)

	return mux
}
