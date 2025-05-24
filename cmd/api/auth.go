package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
	"github.com/sixync/birdlens-be/internal/validator"
)

type LoginUserReq struct {
	Username string `json:"username" validate:"required,min=3,max=20"`
	Password string `json:"password" validate:"required,min=3"`
}

type RefreshTokenReq struct {
	AccessToken  string `json:"access_token" validate:"required"`
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshTokenRes struct {
	AccessToken string `json:"access_token"`
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

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()
	user, err := app.store.Users.GetByUsername(ctx, req.Username)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.invalidCredentials(w, r)
			return
		default:
			app.serverError(w, r, err)
			return
		}
	}

	if matched := utils.CheckPasswordHash(req.Password, *user.HashedPassword); !matched {
		app.invalidCredentials(w, r)
		return
	}

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

	err = app.handleUserSession(ctx, user, refreshToken, refreshTokenExp)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

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

func (app *application) registerHandler(w http.ResponseWriter, r *http.Request) {
	var req auth.RegisterUserReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	// Validate the request
	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()

	// Check if user already UsernameExists
	exists, msg, err := app.validRegisterUserReq(ctx, req)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if exists {
		app.errorMessage(w, r, http.StatusBadRequest, msg, nil)
		return
	}

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
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.errorMessage(w, r, http.StatusBadRequest, "invalid credentials", nil)
			return
		default:
			app.serverError(w, r, err)
			return
		}
	}

	response.JSON(w, http.StatusCreated, user, false, "register successfully")
}

func (app *application) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshTokenReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()
	claims, err := app.tokenMaker.GetUserClaimsFromToken(req.AccessToken)
	if err != nil {
		log.Println("error getting user claims from token", err)
		app.unauthorized(w, r)
		return
	}

	if claims == nil {
		log.Println("claims is nil")
		app.unauthorized(w, r)
		return
	}

	session, err := app.store.Sessions.GetById(ctx, claims.ID)
	if err != nil {
		app.unauthorized(w, r)
		return
	}

	if session.IsRevoked || session.ExpiresAt.Before(time.Now()) {
		log.Println("session is revoked or expired")
		app.unauthorized(w, r)
		return
	}

	accessToken, _, err := app.tokenMaker.CreateToken(claims.ID, claims.Username, app.config.jwt.accessTokenExpDurationMin)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	res := &RefreshTokenRes{
		AccessToken: accessToken,
	}

	response.JSON(w, http.StatusAccepted, res, false, "refresh token successfully")
}

func (app *application) handleUserSession(ctx context.Context, user *store.User, refreshToken string, refreshTokenExp time.Time) error {
	log.Println("hit handleUserSession with user, refreshToken, refreshTokenExp", user, refreshToken, refreshTokenExp)
	s, err := app.store.Sessions.GetById(ctx, user.Id)

	// Create if not found and add expire if found
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Println("create new session")
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
	} else { // Update if found
		log.Println("update session with session is", s)
		log.Println("this is refreshTokenExp", refreshTokenExp)
		log.Println("this is s.expireat", s.ExpiresAt)
		s.ExpiresAt = refreshTokenExp
		s.RefreshToken = refreshToken
		log.Println("session after update", s)
		err := app.store.Sessions.UpdateSession(ctx, s)
		if err != nil {
			return err
		}
	}

	return nil
}

func (app *application) validRegisterUserReq(ctx context.Context, req auth.RegisterUserReq) (exists bool, msg string, err error) {
	con1, err := app.store.Users.EmailExists(ctx, req.Email)
	if con1 {
		msg += "email already exists\n"
	}
	if err != nil {
		return false, msg, err
	}

	con2, err := app.store.Users.UsernameExists(ctx, req.Username)
	if con2 {
		msg += "username already exists\n"
	}
	if err != nil {
		return false, msg, err
	}

	return con1 || con2, msg, nil
}
