package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/auth"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	userID := flag.Uint("user-id", 1, "user ID to embed in the token")
	role := flag.String("role", "customer", "role to embed in the token (admin or customer)")
	flag.Parse()

	token, err := auth.GenerateJWTToken(uint(*userID), auth.UserRole(*role))
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(token)
}
