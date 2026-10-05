package handler

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/Hiroki111/go-carshop-backend/order-service/internal/service"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	service   *service.Service
	validate  *validator.Validate
	publicKey *rsa.PublicKey
}

func NewHandler(service *service.Service, publicKey *rsa.PublicKey) *Handler {
	v := validator.New()

	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	return &Handler{
		service:   service,
		validate:  v,
		publicKey: publicKey,
	}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}
