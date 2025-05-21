package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

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
		HashedPassword: hashedPassword,
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
	claims := app.getUserClaimsFromCtx(r)

	profile, err := app.store.Users.GetById(r.Context(), claims.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusOK, profile, false, "get successful"); err != nil {
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
