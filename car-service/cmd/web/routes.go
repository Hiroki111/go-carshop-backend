package main

import (
	"net/http"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/Hiroki111/go-carshop-backend/car-service/docs"
	httpSwagger "github.com/swaggo/http-swagger"
)

func routes(handler *handler.Handler) http.Handler {
	mux := chi.NewRouter()

	mux.Route("/", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		r.Get("/cars", handler.GetProducts)
		r.Get("/cars/{id}", handler.GetProductById)
		r.Post("/cars", handler.CreateProduct)
		r.Patch("/cars/{id}", handler.UpdateProduct)
		r.Delete("/cars/{id}", handler.DeleteProduct)
	})

	// public
	mux.Get("/ping", handler.Ping)
	mux.Handle("/metrics", promhttp.Handler())

	// Swagger route
	mux.Get("/swagger/*", httpSwagger.WrapHandler)

	return mux
}
