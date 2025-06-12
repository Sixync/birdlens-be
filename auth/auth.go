package auth

import (
	"context"
	"database/sql"
	"errors"
	"log"

	"firebase.google.com/go/auth"
	"github.com/google/uuid"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
)

var ErrMailNotVerified = errors.New("email not verified")
var ErrUserNotFound = errors.New("user not found")
var ErrIncorrectPassword = errors.New("incorrect password")

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
	log.Println("Attempting to login user with info:", email, password, subscription)
	user, err := s.store.Users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Println("user not found with email:", email)
			return "", ErrUserNotFound 
		}
		log.Println("error getting user by email:", err)
		return "", err 
	}

	if user == nil {
		log.Println("user is nil after GetByEmail, though no explicit error was returned. This indicates user not found for email:", email)
		return "", ErrUserNotFound 
	}


	log.Printf("user found: %+v", user) 

	if !user.EmailVerified {
		log.Println("user email not verified for user:", user.Email)
		return "", ErrMailNotVerified
	}

	if user.HashedPassword == nil {
		log.Println("user has no hashed password set:", user.Email)
		return "", errors.New("user account is not properly configured for password login")
	}

	if matched := utils.CheckPasswordHash(password, *user.HashedPassword); !matched {
		log.Println("incorrect password for user:", user.Email)
		return "", ErrIncorrectPassword
	}

	claims := map[string]any{
		"username":     user.Username,
		"subscription": subscription,
	}
    if user.FirebaseUID == nil || *user.FirebaseUID == "" {
        log.Printf("User %s (ID: %d) is missing FirebaseUID. Cannot generate custom token.", user.Email, user.Id)
        return "", errors.New("internal server error: user account configuration issue")
    }


	token, err := s.FireAuth.CustomTokenWithClaims(ctx, *user.FirebaseUID, claims)
	if err != nil {
		log.Printf("failed to generate custom token for UID %s: %v", *user.FirebaseUID, err)
		return "", errors.New("internal server error generating auth token")
	}

	log.Printf("generated custom token for UID %s", *user.FirebaseUID)

	return token, nil
}

func (s *AuthService) Register(ctx context.Context, req RegisterUserReq) (string, int64, error) {
	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return "", 0, err
	}

	user := req.toUser()

	uid := uuid.New().String()
	user.FirebaseUID = &uid
	log.Println("Creating user with Firebase UID:", *user.FirebaseUID)

	user.HashedPassword = &hashedPassword

	if req.AuthProvider == "" {
		req.AuthProvider = "firebase"
	}
	user.AuthProvider = req.AuthProvider
	user.EmailVerified = false 

	log.Println("user hashed password:", *user.HashedPassword)

	err = s.store.Users.Create(ctx, user) 
	if err != nil {
		log.Printf("failed to create user in database: %v", err)
		return "", 0, errors.New("failed to register user")
	}


	customToken, err := s.FireAuth.CustomToken(ctx, *user.FirebaseUID)
	if err != nil {
		log.Printf("failed to create custom token for user %s (UID: %s): %v", user.Email, *user.FirebaseUID, err)
		return "", 0, errors.New("internal server error creating auth token post-registration")
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