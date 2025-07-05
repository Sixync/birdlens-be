// birdlens-be/cmd/api/users.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
)

var UserKey key = "user"

func (app *application) getUserFollowersHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromCtx(r)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	ctx := r.Context()
	result, err := app.store.Followers.GetByUserId(ctx, user.Id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusOK, result, false, "get successful"); err != nil {
		app.serverError(w, r, err)
	}
}

// New handler for the life list
func (app *application) getUserLifeListHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromCtx(r)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	ctx := r.Context()
	lifeList, err := app.store.Users.GetUserLifeList(ctx, user.Id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, lifeList, false, "life list retrieved successfully")
}

type CreateUserReq struct {
	Username  string  `json:"username"`
	Password  string  `json:"password"`
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Age       int     `json:"age"`
	AvatarUrl *string `json:"avatar_url"`
}

type UserResponse struct {
	Username      string  `json:"username"`
	FirstName     string  `json:"first_name"`
	LastName      string  `json:"last_name"`
	Email         string  `json:"email"`
	Age           int     `json:"age"`
	AvatarUrl     *string `json:"avatar_url"`
	Subscription  string  `json:"subscription,omitempty"`
	EmailVerified bool    `json:"email_verified"` // Added this field
}

func (app *application) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	user := &store.User{
		Username:       req.Username,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Email:          req.Email,
		Age:            req.Age,
		AvatarUrl:      req.AvatarUrl,
		HashedPassword: &hashedPassword,
	}
	err = app.store.Users.Create(ctx, user)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Note: The createUserHandler currently returns the store.User object directly.
	// If it were to return UserResponse, EmailVerified would need to be populated here too.
	// However, this endpoint is less critical for this specific flow than /users/me.
	if err := response.JSON(w, http.StatusCreated, user, false, "create successful"); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) getUserHandler(w http.ResponseWriter, r *http.Request) {
	user, err := getUserFromCtx(r)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	// This handler would also need to be updated if it were to return UserResponse
	// to include the EmailVerified field from the store.User.
	if err := response.JSON(w, http.StatusOK, user, false, "get successful"); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) getCurrentUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	userFromClaims := app.getUserFromFirebaseClaimsCtx(r) // This fetches user from DB based on Firebase UID

	log.Println("getCurrentUserProfileHandler user from claims context (includes DB data):", userFromClaims)

	if userFromClaims == nil {
		app.unauthorized(w, r)
		return
	}
	ctx := r.Context()

	// The userFromClaims already contains the EmailVerified status from the database
	// because getUserFromFirebaseClaimsCtx calls store.Users.GetByFirebaseUID which selects all fields.

	subscription, err := app.store.Subscriptions.GetUserSubscriptionByEmail(ctx, userFromClaims.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, r, err)
		return
	}

	log.Printf("get user sub: %+v", subscription)

	res := &UserResponse{
		Username:      userFromClaims.Username,
		FirstName:     userFromClaims.FirstName,
		LastName:      userFromClaims.LastName,
		Email:         userFromClaims.Email,
		Age:           userFromClaims.Age,
		AvatarUrl:     userFromClaims.AvatarUrl,
		Subscription:  "",
		EmailVerified: userFromClaims.EmailVerified, // Populate the new field
	}

	if subscription != nil {
		res.Subscription = subscription.Name
	}

	if err := response.JSON(w, http.StatusOK, res, false, "get successful"); err != nil {
		app.serverError(w, r, err)
	}
}

func getUserFromCtx(r *http.Request) (*store.User, error) {
	ctx := r.Context()
	user, _ := ctx.Value(UserKey).(*store.User)
	return user, nil
}

func (app *application) getUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userId := r.PathValue("user_id")
		if userId == "" {
			app.badRequest(w, r, errors.New("user_id is required"))
			return
		}

		userIdInt, err := strconv.ParseInt(userId, 10, 64)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		user, err := app.store.Users.GetById(r.Context(), userIdInt) // GetById needs to select EmailVerified
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) getUserFromFirebaseClaimsCtx(r *http.Request) *store.User {
	claims, ok := r.Context().Value(UserClaimsKey).(*jwt.FirebaseClaims)
	if !ok {
		return nil
	}
	if claims == nil {
		log.Println("claims is nil")
		return nil
	}

	log.Println("getUserFromFirebaseClaimsCtx claims:", claims)

	ctx := r.Context()
	// GetByFirebaseUID should select all necessary fields including EmailVerified
	user, err := app.store.Users.GetByFirebaseUID(ctx, claims.Uid)
	if err != nil {
		log.Printf("failed to get user by Firebase UID %s: %v", claims.Uid, err)
		return nil
	}

	return user
}