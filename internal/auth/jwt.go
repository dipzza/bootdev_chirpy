package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const expireTime = time.Second * 3600

func MakeJWT(userID uuid.UUID, tokenSecret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expireTime)),
		Subject:   userID.String(),
	})

	return token.SignedString([]byte(tokenSecret))
}

func ValidateJWT(tokenString string, tokenSecret string) (uuid.UUID, error) {
	claims := jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.UUID{}, err
	}

	return userID, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	bearerToken := headers.Get("Authorization")
	if len(bearerToken) == 0 {
		return "", fmt.Errorf("no authorization header found")
	}

	if !strings.HasPrefix(bearerToken, "Bearer ") {
		return "", fmt.Errorf("invalid authorization header")
	}

	return strings.TrimPrefix(bearerToken, "Bearer "), nil
}