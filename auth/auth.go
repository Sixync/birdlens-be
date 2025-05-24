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

var (
	ErrEmailExists    error = errors.New("email already used")
	ErrUsernameExists error = errors.New("username already used")
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

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	// Get the user from the database
	user, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		return "", err
	}

	// Check if the provided password matches the user's password
	if matched := utils.CheckPasswordHash(password, *user.HashedPassword); matched {
		return "", errors.New("incorrect password")
	}

	// Generate a Firebase custom token for the user
	token, err := s.FireAuth.CustomToken(context.Background(), *user.FirebaseUID)
	if err != nil {
		log.Printf("failed to generate custom token: %v", err)
		return "", errors.New("internal server error")
	}

	return token, nil
}

// Register creates a new user with the provided credentials and returns token
func (s *AuthService) Register(ctx context.Context, req RegisterUserReq) (string, error) {
	// Check if the user with the email already exists
	exists, err := s.store.Users.EmailExists(ctx, req.Email)
	if err != nil {
		return "", err
	}

	if exists {
		return "", ErrEmailExists
	}

	// Generate a UUID for the new user

	// Generate a hash of the user's password using bcrypt
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return "", err
	}

	// Create a new user in the database
	user := req.toUser()
	*user.FirebaseUID = uuid.New().String()
	user.HashedPassword = &hashedPassword

	s.store.Users.Create(ctx, user)

	// Create a custom token for the user using the Firebase Admin SDK
	customToken, err := s.FireAuth.CustomToken(ctx, *user.FirebaseUID)
	if err != nil {
		log.Printf("failed to create custom token for user: %v", err)
		return "", errors.New("internal server error")
	}

	return customToken, nil
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
