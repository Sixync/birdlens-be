package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTMaker struct {
	secretKey string
}

func NewJWTMaker(secretKey string) *JWTMaker {
	return &JWTMaker{
		secretKey: secretKey,
	}
}

func (maker *JWTMaker) CreateToken(userID int64, username string, durationMin int) (string, *UserClaims, error) {
	// Implement token creation logic here
	duration := time.Duration(durationMin) * time.Minute
	claims, err := NewUserClaims(userID, username, "", nil, duration)
	if err != nil {
		return "", nil, err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(maker.secretKey))
	if err != nil {
		return "", nil, err
	}

	return tokenStr, claims, nil
}

func (maker *JWTMaker) VerifyToken(tokenStr string) (*UserClaims, error) {
	// Implement token verification logic here
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (any, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)

		if !ok {
			return nil, errors.New("error parsing token")
		}
		return []byte(maker.secretKey), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token claims")
}

func (maker *JWTMaker) CreateRandomToken() (string, error) {
	// Generate a secure random token
	tokenBytes := make([]byte, 32)
	_, err := rand.Read(tokenBytes)
	if err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	return token, nil
}

func (maker *JWTMaker) GetUserClaimsFromToken(tokenStr string) (*UserClaims, error) {
	// Parse the token and extract claims
	log.Println("hit GetUserClaimsFromToken with tokenStr", tokenStr)
	token, err := jwt.ParseWithClaims(tokenStr, &UserClaims{}, func(token *jwt.Token) (any, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(maker.secretKey), nil
	})
	log.Println("token", token)
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return nil, err
	}

	if claims := token.Claims.(*UserClaims); claims != nil {
		log.Println("claims", claims)
		return claims, nil
	}

	log.Println("claims is nil")

	return nil, errors.New("invalid token claims")
}
