package jwt

import (
	"fmt"
	"time"

	"firebase.google.com/go/auth"
	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type UserClaims struct {
	ID       int64    `json:"id"`
	Username string   `json:"username"`
	Email    string   `json:"email"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

type FirebaseClaims struct {
	Iss string `json:"iss"`
	Aud string `json:"aud"`
	Exp int64  `json:"exp"`
	Iat int64  `json:"iat"`
	Sub string `json:"sub"`
	Uid string `json:"uid"`
}

func NewUserClaims(id int64, username, email string, roles []string, duration time.Duration) (*UserClaims, error) {
	tokenId, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("error generating token ID: %w", err)
	}
	return &UserClaims{
		ID:       id,
		Username: username,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenId.String(),
			Subject:   email,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(duration)),
		},
	}, nil
}

func NewFirebaseClaims(t *auth.Token) *FirebaseClaims {
	return &FirebaseClaims{
		Iss: t.Issuer,
		Aud: t.Audience,
		Exp: t.Expires,
		Iat: t.IssuedAt,
		Sub: t.Subject,
		Uid: t.UID,
	}
}
