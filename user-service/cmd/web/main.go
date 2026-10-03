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

	"github.com/Hiroki111/go-carshop-backend/user-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/user-service/internal/service"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const portNumber = ":8082"

// @title           Go User Service API
// @version         1.0
// @description     Issues and manages user accounts and login tokens for go-carshop-backend.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8082
// @BasePath  /
func main() {
	// NOTE: Ignore error; variables might be injected by Docker/K8s
	_ = godotenv.Load()

	privateKeyPath := os.Getenv("PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		log.Fatal("PRIVATE_KEY_PATH not set")
	}

	privateKey, err := auth.LoadPrivateKey(privateKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	adminUserName := os.Getenv("ADMIN_USERNAME")
	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminUserName == "" || adminPassword == "" {
		log.Fatal("ADMIN_USERNAME and ADMIN_PASSWORD must both be set")
	}

	db, err := newPostgresDB()
	if err != nil {
		log.Fatal(err)
	}

	repo := repository.NewRepository(db)
	if err := repo.Migrate(); err != nil {
		log.Fatal(err)
	}

	hashedAdminPassword, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	if err := repo.EnsureAdminExists(adminUserName, string(hashedAdminPassword)); err != nil {
		log.Fatal(err)
	}

	svc := service.NewService(repo, privateKey)
	h := handler.NewHandler(svc)

	server := &http.Server{
		Addr:    portNumber,
		Handler: routes(h),
	}

	// Channel that listens for OS signals
	shutdownCh := make(chan os.Signal, 1)
	signal.Notify(shutdownCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownCh)

	// Start server in a goroutine
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
	fmt.Println("Closing database connections...")

	if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}

	fmt.Println("Server exited properly")
}

func newPostgresDB() (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=%s",
		getEnv("DB_HOST"),
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_NAME"),
		getEnv("DB_PORT"),
		getEnv("DB_SSLMODE"),
		getEnv("DB_TIMEZONE"),
	)
	return gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
}

func getEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	panic(fmt.Sprintf("Env variable %s not found", key))
}
