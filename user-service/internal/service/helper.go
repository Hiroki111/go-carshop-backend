package service

import (
	"strings"
)

func NormalizeEmail(email string) string {
	return strings.ToLower(email)
}
