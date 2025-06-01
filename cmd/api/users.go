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
	Username     string  `json:"username"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Email        string  `json:"email"`
	Age          int     `json:"age"`
	AvatarUrl    *string `json:"avatar_url"`
	Subscription string  `json:"subscription,omitempty"`
}

func (app *application) createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()

	// Hash the password
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

	if err := response.JSON(w, http.StatusOK, user, false, "get successful"); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) getCurrentUserProfileHandler(w http.ResponseWriter, r *http.Request) {
	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}
	ctx := r.Context()

	profile, err := app.store.Users.GetById(ctx, user.Id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	subscription, err := app.store.Subscriptions.GetUserSubscriptionByEmail(ctx, profile.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, r, err)
		return
	}

	log.Printf("get user sub: %+v", subscription)

	res := &UserResponse{
		Username:     profile.Username,
		FirstName:    profile.FirstName,
		LastName:     profile.LastName,
		Email:        profile.Email,
		Age:          profile.Age,
		AvatarUrl:    profile.AvatarUrl,
		Subscription: "",
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

		user, err := app.store.Users.GetById(r.Context(), userIdInt)
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
		// This should not happen if middleware is correctly applied
		// and always sets the claims. Could panic or log an error.
		return nil
	}
	if claims == nil {
		log.Println("claims is nil")
		return nil
	}

	ctx := r.Context()
	user, err := app.store.Users.GetByFirebaseUID(ctx, claims.Uid)
	if err != nil {
		log.Printf("failed to get user by Firebase UID %s: %v", claims.Uid, err)
		return nil
	}

	return user
}
