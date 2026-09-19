package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
)

const portNumber = ":8080"

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", healthHandler)

	h := handler.NewHandler()

	server := &http.Server{
		Addr:    portNumber,
		Handler: routes(h),
	}

	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownCh)

	go func() {
		fmt.Printf("Starting application on port %s\n", portNumber)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Block until signal received
	<-shutdownCh
	fmt.Println("Shutting down server...")

	// 1. Create shutdown context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 2. Stop accepting new requests and wait for active ones to finish
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}

	// 3. Now that no more handlers are running, close infra
	fmt.Println("Closing database and cache connections...")
	// TODO: Close DB, redis client, etc when infra-related services are introduced

	fmt.Println("Server exited properly")
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintln(w, `{"status":"ok"}`)
}
