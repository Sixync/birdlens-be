package auth

import (
	"context"
	"errors"
	"log"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
)

type AuthService struct {
	store    *store.Storage
	FireAuth *auth.Client
}

func NewAuthService(s *store.Storage, c *auth.Client) *AuthService {
	return &AuthService{
		store:    s,
		FireAuth: c,
	}
}

type RegisterUserReq struct {
	Username     string  `json:"username" validate:"required,min=3,max=20"`
	Password     string  `json:"password" validate:"required,min=3"`
	Email        string  `json:"email" validate:"required,email"`
	FirstName    string  `json:"first_name" validate:"required,min=3,max=20"`
	LastName     string  `json:"last_name" validate:"required,min=3,max=20"`
	Age          int     `json:"age" validate:"required,min=1,max=120"`
	AvatarUrl    *string `json:"avatar_url"`
	AuthProvider string  `json:"auth_provider"`
}

func (s *AuthService) Login(ctx context.Context,
	email,
	password,
	subscription string,
) (string, error) {
	// Get the user from the database
	log.Println("Attempting to login user with info:", email, password, subscription)
	user, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	log.Printf("user found: %v", user)

	// Check if the provided password matches the user's password
	if matched := utils.CheckPasswordHash(password, *user.HashedPassword); !matched {
		return "", errors.New("incorrect password")
	}

	// Generate a Firebase custom token for the user
	claims := map[string]any{
		"username":     user.Username,
		"subscription": subscription,
	}

	token, err := s.FireAuth.CustomTokenWithClaims(ctx, *user.FirebaseUID, claims)
	if err != nil {
		log.Printf("failed to generate custom token: %v", err)
		return "", errors.New("internal server error")
	}

	log.Printf("generated custom token: %s", token)

	return token, nil
}

// Register creates a new user with the provided credentials and returns token
func (s *AuthService) Register(ctx context.Context, req RegisterUserReq) (string, int64, error) {
	// Generate a hash of the user's password using bcrypt
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return "", 0, err
	}

	// Create a new user in the database
	user := req.toUser()

	uid := uuid.New().String()

	user.FirebaseUID = &uid

	log.Println("Creating user with Firebase UID:", *user.FirebaseUID)

	user.HashedPassword = &hashedPassword

	if req.AuthProvider == "" {
		req.AuthProvider = "firebase"
	}

	user.AuthProvider = req.AuthProvider

	log.Println("user hashed password:", *user.HashedPassword)

	s.store.Users.Create(ctx, user)

	// Create a custom token for the user using the Firebase Admin SDK
	customToken, err := s.FireAuth.CustomToken(ctx, *user.FirebaseUID)
	if err != nil {
		log.Printf("failed to create custom token for user: %v", err)
		return "", 0, errors.New("internal server error")
	}

	return customToken, user.Id, nil
}

func (req *RegisterUserReq) toUser() *store.User {
	return &store.User{
		Username:     req.Username,
		Age:          req.Age,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        req.Email,
		AvatarUrl:    req.AvatarUrl,
		AuthProvider: req.AuthProvider,
	}
}
