package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/carclient"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/handler"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/kafkapublisher"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/outbox"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/repository"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
	"github.com/Hiroki111/go-carshop-backend/order-service/internal/serviceauth"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	portNumber                 = ":8081"
	ordersTopic                = "orders"
	initialProductListCacheTTL = 30 * time.Minute
)

// @title           Go Backend Example API
// @version         1.0
// @description     This is a sample order server.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8081
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and then your token.
func main() {
	// NOTE: Ignore error; variables might be injected by Docker/K8s
	_ = godotenv.Load()

	publicKeyPath := os.Getenv("PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		log.Fatal("PUBLIC_KEY_PATH not set")
	}
	publicKey, err := auth.LoadPublicKey(publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	carServiceURL := os.Getenv("CAR_SERVICE_URL")
	if carServiceURL == "" {
		log.Fatal("CAR_SERVICE_URL not set")
	}

	tokens, err := newServiceTokenSource()
	if err != nil {
		log.Fatal(err)
	}

	db, err := newPostgresDB()
	if err != nil {
		log.Fatal(err)
	}

	repo := repository.NewRepository(db)
	if err := repo.Migrate(); err != nil {
		log.Fatal(err)
	}

	carClient := carclient.NewHTTPCarClient(carServiceURL, tokens)

	service := service.NewService(repo, carClient)
	h := handler.NewHandler(service, publicKey)

	publisher, err := newKafkaPublisher()
	if err != nil {
		log.Fatal(err)
	}

	// The relay sends the events that orders record in the outbox table to
	// Kafka. It runs next to the server and is stopped before the publisher
	// and the database are closed.
	relay := outbox.NewRelay(repo, publisher, outbox.Config{})
	relayCtx, stopRelay := context.WithCancel(context.Background())
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		_ = relay.Run(relayCtx)
	}()

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

	// 3. Stop the relay (it uses the publisher and the database), then the
	// publisher
	stopRelay()
	select {
	case <-relayDone:
	case <-ctx.Done():
		log.Printf("outbox relay did not stop in time")
	}
	publisher.Close()

	// 4. Now that no more handlers are running, close infra
	fmt.Println("Closing database and cache connections...")

	if sqlDB, err := db.DB(); err == nil && sqlDB != nil {
		_ = sqlDB.Close()
	}

	fmt.Println("Server exited properly")
}

// newServiceTokenSource builds the client that gets this service's tokens from
// user-service (OAuth2 client credentials). The secret has no default: without
// it the service refuses to start rather than fail on the first order.
func newServiceTokenSource() (*serviceauth.ClientCredentials, error) {
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		return nil, fmt.Errorf("USER_SERVICE_URL not set")
	}
	clientID := os.Getenv("OAUTH_CLIENT_ID")
	if clientID == "" {
		return nil, fmt.Errorf("OAUTH_CLIENT_ID not set")
	}
	clientSecret := os.Getenv("OAUTH_CLIENT_SECRET")
	if clientSecret == "" {
		return nil, fmt.Errorf("OAUTH_CLIENT_SECRET not set (it must equal ORDER_SERVICE_CLIENT_SECRET in user-service)")
	}

	return serviceauth.NewClientCredentials(serviceauth.Config{
		TokenURL:     strings.TrimRight(userServiceURL, "/") + "/oauth/token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"cars:reserve", "cars:release"},
	})
}

// newKafkaPublisher builds the publisher for the orders topic. KAFKA_BROKERS
// has no default: without it the service refuses to start instead of silently
// never announcing any order.
func newKafkaPublisher() (*kafkapublisher.Publisher, error) {
	var brokers []string
	for _, broker := range strings.Split(os.Getenv("KAFKA_BROKERS"), ",") {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	if len(brokers) == 0 {
		return nil, fmt.Errorf("KAFKA_BROKERS not set (comma-separated, e.g. localhost:29092)")
	}

	return kafkapublisher.New(brokers, ordersTopic)
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
