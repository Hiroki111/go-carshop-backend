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

	"github.com/Hiroki111/go-carshop-backend/car-service/internal/cache"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/metrics"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/car-service/internal/service"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	portNumber                 = ":8080"
	initialCarListCacheTTL = 30 * time.Minute
)

// @title           Go Backend Example API
// @version         1.0
// @description     This is a sample car ordering server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and then your token.
func main() {
	// NOTE: Ignore error; variables might be injected by Docker/K8s
	_ = godotenv.Load()

	metrics.Register()

	db, err := newPostgresDB()
	if err != nil {
		log.Fatal(err)
	}

	redisClient, err := newRedisClient()
	if err != nil {
		log.Fatal(err)
	}

	carsCache := cache.NewRedisCarsCache(
		redisClient,
	)

	repo := repository.NewRepository(db)
	if err := repo.Migrate(); err != nil {
		log.Fatal(err)
	}

	carsCacheWarmer := cache.NewRedisCarsCacheWarmer(*repo, carsCache)
	carsCacheWarmer.WarmCarList(initialCarListCacheTTL)

	service := service.NewService(repo, carsCache, carsCacheWarmer)
	h := handler.NewHandler(service)

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
	fmt.Println("Closing database and cache connections...")

	if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}

	if err := redisClient.Close(); err != nil {
		log.Printf("failed to close redis client: %v", err)
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

func newRedisClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", getEnv("REDIS_HOST"), getEnv("REDIS_PORT")),
		Password: getEnv("REDIS_PASSWORD"),
	})

	ctx, cancle := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancle()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

func getEnv(key string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	panic(fmt.Sprintf("Env variable %s not found", key))
}
