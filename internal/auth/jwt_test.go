package auth

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWT(t *testing.T) {
	ogUserId := uuid.New()
	secret := "secret"

	tests := []struct {
		name 				 string
		tokenSecret  string
		expiresIn    time.Duration
		expectedId   uuid.UUID
		expectedErr  error
	}{
		{
			name:         "Valid token",
			tokenSecret:  secret,
			expiresIn:    20 * time.Minute,
			expectedId:   ogUserId,
			expectedErr:  nil,
		},
		{
			name:         "Expired token",
			tokenSecret:  secret,
			expiresIn:    0 * time.Second,
			expectedId:   uuid.UUID{},
			expectedErr:  fmt.Errorf("token is expired"),
		},
		{
			name:         "Signed with wrong secret",
			tokenSecret:  "wrong secret",
			expiresIn:    20 * time.Minute,
			expectedId:   uuid.UUID{},
			expectedErr:  fmt.Errorf("token signature is invalid"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenString, err := MakeJWT(ogUserId, tt.tokenSecret, tt.expiresIn)
			if err != nil {
				t.Errorf("Error creating JWT: %v", err)
			}
			userId, err := ValidateJWT(tokenString, secret)

			correctErr := (err == nil && tt.expectedErr == nil) || strings.Contains(err.Error(), tt.expectedErr.Error())
			correctResult := tt.expectedId == userId

			if !correctErr {
				t.Errorf("ValidateJWT() error = %v, expectedErr %v", err, tt.expectedErr)
			}
			if !correctResult {
				t.Errorf("ValidateJWT() userId %v, expectedUserId %v", userId, tt.expectedId)
			}
		})
	}
}