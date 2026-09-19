package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func parseOptionalInt64(value string, defaultValue int64) (int64, error) {
	if value == "" {
		return defaultValue, nil
	}
	return strconv.ParseInt(value, 10, 64)
}

func parseOptionalInt(value string, defaultValue int) (int, error) {
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}

func (h *Handler) formatValidationError(err error) string {
	errorMessages := make([]string, 0)
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, validationError := range validationErrors {
			switch validationError.Tag() {
			case "required":
				errorMessages = append(errorMessages, fmt.Sprintf("field '%s' is required", validationError.Field()))
			case "min":
				errorMessages = append(errorMessages, fmt.Sprintf("field '%s' must be at least %s characters long", validationError.Field(), validationError.Param()))
			case "max":
				errorMessages = append(errorMessages, fmt.Sprintf("field '%s' cannot exceed %s characters", validationError.Field(), validationError.Param()))
			case "gt":
				errorMessages = append(errorMessages, fmt.Sprintf("field '%s' must be greater than %s", validationError.Field(), validationError.Param()))
			}
		}
	}

	if len(errorMessages) == 0 {
		return "invalid parameter found"
	}
	return strings.Join(errorMessages, ", ")
}
