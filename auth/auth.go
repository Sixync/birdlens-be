package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

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
	Username     string  `json:"-"`
	Password     string  `json:"password" validate:"required,min=3"`
	Email        string  `json:"email" validate:"required,email"`
	ReferralCode *string `json:"referral_code,omitempty"`
	FirstName    string  `json:"-"`
	LastName     string  `json:"-"`
	Age          int     `json:"-"`
	AvatarUrl    *string `json:"-"`
	AuthProvider string  `json:"-"`
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
	emailParts := strings.Split(req.Email, "@")
	baseUsername := emailParts[0]

	reg, err := regexp.Compile("[^a-zA-Z0-9_]+")
	if err != nil {
		log.Printf("failed to compile username sanitization regex: %v", err)
		return "", 0, errors.New("internal server error during registration")
	}
	baseUsername = reg.ReplaceAllString(baseUsername, "_")

	if len(baseUsername) > 15 {
		baseUsername = baseUsername[:15]
	}

	var finalUsername string
	for i := 0; i < 5; i++ {
		tempUsername := baseUsername
		if i > 0 {
			randomSuffix := uuid.New().String()[:4]
			tempUsername = fmt.Sprintf("%s_%s", baseUsername, randomSuffix)
		}

		exists, err := s.store.Users.UsernameExists(ctx, tempUsername)
		if err != nil {
			return "", 0, err
		}
		if !exists {
			finalUsername = tempUsername
			break
		}
	}

	if finalUsername == "" {
		return "", 0, errors.New("could not generate a unique username")
	}

	req.Username = finalUsername
	req.FirstName = "New"
	req.LastName = "User"
	req.Age = 18

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
    
	if req.ReferralCode != nil && *req.ReferralCode != "" {
		log.Printf("Processing referral code '%s' for new user %d", *req.ReferralCode, user.Id)
		referrer, err := s.store.Users.GetByUsername(ctx, *req.ReferralCode)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				log.Printf("Referral code '%s' is not a valid user. Ignoring.", *req.ReferralCode)
			} else {
				log.Printf("Error checking referrer user: %v. Ignoring referral.", err)
			}
		} else {
			newReferral := &store.Referral{
				ReferrerID:       referrer.Id,
				RefereeID:        user.Id,
				Status:           store.ReferralStatusPending,
				ReferralCodeUsed: *req.ReferralCode,
			}
			err = s.store.Referrals.Create(ctx, newReferral)
			if err != nil {
				log.Printf("Failed to create referral record for new user %d referred by %d: %v", user.Id, referrer.Id, err)
			} else {
				log.Printf("Successfully created pending referral record ID %d", newReferral.ID)
			}
		}
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