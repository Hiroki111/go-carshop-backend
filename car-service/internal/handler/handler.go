package handler

import (
	"fmt"
	"net/http"

	"github.com/go-playground/validator/v10"
)

type Handler struct {
	validate *validator.Validate
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "pong")
}
