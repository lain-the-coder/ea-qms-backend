package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
)

func MakeRefreshToken() (string, error) {
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", fmt.Errorf("error generating random bytes: %w", err)
	}
	return hex.EncodeToString(tokenBytes), nil
}

func GetBearerToken(headers http.Header) (string, error) {
	headerValue := headers.Get("Authorization")
	if headerValue == "" {
		return "", fmt.Errorf("missing Authorization Header")
	}
	rawToken, found := strings.CutPrefix(headerValue, "Bearer ")
	if !found {
		return "", fmt.Errorf("malformed token format")
	}
	token := strings.TrimSpace(rawToken)
	if token == "" {
		return "", fmt.Errorf("empty token value")
	}
	return token, nil
}
