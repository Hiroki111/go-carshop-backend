package main

import (
	"net/http"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/auth"
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

		r.Get("/cars", handler.GetCars)
		r.Get("/cars/{id}", handler.GetCarById)
		r.Post("/cars", handler.RequireRole(auth.AdminRole, handler.CreateCar))
		r.Patch("/cars/{id}", handler.RequireRole(auth.AdminRole, handler.UpdateCar))
		r.Patch("/cars/{id}/status", handler.RequireRole(auth.AdminRole, handler.UpdateCarStatus))
		r.Delete("/cars/{id}", handler.RequireRole(auth.AdminRole, handler.DeleteCar))

		// internal: called by other services with a service token (scope), not by users
		r.Post("/internal/cars/{id}/reservations", handler.RequireScope(auth.ScopeCarsReserve, handler.ReserveCar))
	})

	// public
	mux.Get("/ping", handler.Ping)
	mux.Handle("/metrics", promhttp.Handler())

	// Swagger route
	mux.Get("/swagger/*", httpSwagger.WrapHandler)

	return mux
}
