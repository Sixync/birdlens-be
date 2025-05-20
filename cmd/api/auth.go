package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
)

type LoginUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginUserRes struct {
	SessionID       int64     `json:"session_id"`
	AccessToken     string    `json:"access_token"`
	RefreshToken    string    `json:"refresh_token"`
	AccessTokenExp  time.Time `json:"access_token_exp"`
	RefreshTokenExp time.Time `json:"refresh_token_exp"`
}

func (app *application) loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginUserReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()
	user, err := app.store.Users.GetByUsername(ctx, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.errorMessage(w, r, http.StatusBadRequest, "invalid credentials", nil)
			return
		default:
			app.serverError(w, r, err)
			return
		}
	}

	log.Println("pass get user with user", user)

	if matched := utils.CheckPasswordHash(req.Password, user.HashedPassword); !matched {
		app.unauthorized(w, r)
		return
	}

	log.Println("pass match pass")

	// generate jwt
	durationMin := app.config.jwt.accessTokenExpDurationMin
	token, userClaims, err := app.tokenMaker.CreateToken(user.Id, user.Username, durationMin)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	log.Println("pass create jwt token with token", token)

	// create and store refresh token

	refreshToken, err := app.tokenMaker.CreateRefreshToken()
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	log.Println("pass create refresh token with token", refreshToken)

	refreshTokenDuration := app.config.jwt.refreshTokenExpDurationDay
	refreshTokenExp := time.Now().Add(time.Hour * 24 * time.Duration(refreshTokenDuration))

	app.handleUserSession(ctx, user, refreshToken, refreshTokenExp)

	result := &LoginUserRes{
		SessionID:       user.Id,
		AccessToken:     token,
		RefreshToken:    refreshToken,
		AccessTokenExp:  userClaims.ExpiresAt.Time,
		RefreshTokenExp: refreshTokenExp,
	}

	log.Println("pass create session with result", result)

	response.JSON(w, http.StatusOK, result, false, "login successful")
}

func (app *application) handleUserSession(ctx context.Context, user *store.User, refreshToken string, refreshTokenExp time.Time) error {
	s, err := app.store.Sessions.GetById(ctx, user.Id)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		session := &store.Session{
			ID:           user.Id,
			UserEmail:    user.Email,
			RefreshToken: refreshToken,
			IsRevoked:    false,
			ExpiresAt:    refreshTokenExp,
		}
		err = app.store.Sessions.Create(ctx, session)
		if err != nil {
			return err
		}
	} else {
		s.ExpiresAt = refreshTokenExp
		// TODO: implement the rest because im too lazy
	}

	return nil
}
