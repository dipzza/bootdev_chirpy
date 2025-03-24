package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	authHeader := headers.Get("Authorization")
	if len(authHeader) == 0 {
		return "", fmt.Errorf("no authorization header found")
	}

	if !strings.HasPrefix(authHeader, "ApiKey ") {
		return "", fmt.Errorf("invalid authorization header")
	}

	return strings.TrimPrefix(authHeader, "ApiKey "), nil
}