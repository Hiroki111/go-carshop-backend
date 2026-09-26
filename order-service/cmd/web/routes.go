package main

import (
	"net/http"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	_ "github.com/Hiroki111/go-carshop-backend/order-service/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

func routes(handler *handler.Handler) http.Handler {
	mux := chi.NewRouter()

	mux.Route("/", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		r.Post("/orders", handler.CreateOrder)
		r.Get("/orders", handler.GetOrders)
		r.Get("/orders/{id}", handler.GetOrderById)
		r.Patch("/orders/{id}", handler.UpdateOrder)
		r.Delete("/orders/{id}", handler.DeleteOrder)
	})

	// infra / public routes
	mux.Get("/ping", handler.Ping)

	// Swagger route
	mux.Get("/swagger/*", httpSwagger.WrapHandler)

	return mux
}
