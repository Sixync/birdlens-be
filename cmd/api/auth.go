package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/validator"
)

type LoginUserReq struct {
	Email    string `json:"email" validate:"required,email"`
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

	var subParam string

	subscription, err := app.store.Subscriptions.GetUserSubscriptionByEmail(ctx, req.Email)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		app.serverError(w, r, err)
		return
	}

	if subscription != nil {
		subParam = subscription.Name
	}

	customToken, err := app.authService.Login(ctx, req.Email, req.Password, subParam)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrMailNotVerified):
			log.Println("user email not verified")
			app.badRequest(w, r, errors.New("email not verified"))
		default:
			log.Println("error logging in user with email", req.Email, "and error", err)
			app.serverError(w, r, err)
		}
		return
	}

	// generate jwt
	// durationMin := app.config.jwt.accessTokenExpDurationMin
	// token, userClaims, err := app.tokenMaker.CreateToken(user.Id, user.Username, durationMin)
	// if err != nil {
	// 	app.serverError(w, r, err)
	// 	return
	// }

	// create and store refresh token

	// refreshToken, err := app.tokenMaker.CreateRefreshToken()
	// if err != nil {
	// 	app.serverError(w, r, err)
	// 	return
	// }
	// log.Println("pass create refresh token with token", refreshToken)
	//
	// refreshTokenDuration := app.config.jwt.refreshTokenExpDurationDay
	// refreshTokenExp := time.Now().Add(time.Hour * 24 * time.Duration(refreshTokenDuration))
	//
	// err = app.handleUserSession(ctx, user, refreshToken, refreshTokenExp)
	// if err != nil {
	// 	app.serverError(w, r, err)
	// 	return
	// }
	//
	log.Println("pass create session with result", customToken)

	response.JSON(w, http.StatusOK, customToken, false, "login successful")
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

	valid, msg, err := app.validRegisterUserReq(ctx, req)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if !valid {
		log.Println("invalid register user req with valid", valid, "and msg", msg)
		app.badRequest(w, r, errors.New(msg))
		return
	}

	log.Println("valid register user req with valid", valid, "and msg", msg)

	customerToken, userId, err := app.authService.Register(ctx, req)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	// send verification email
	// create token
	emailToken, err := app.tokenMaker.CreateRandomToken()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	duration := time.Duration(app.config.emailVerificationExpiresInHours) * time.Hour
	expiresAt := time.Now().Add(duration)

	app.store.Users.AddEmailVerificationToken(ctx, userId, emailToken, expiresAt)

	// store email token in the database

	links := url.Values{
		"token":   []string{emailToken},
		"user_id": []string{strconv.FormatInt(userId, 10)},
	}
	activationURL := app.config.frontEndUrl + "/auth/confirm-email?" + links.Encode()

	log.Println("activationURL", activationURL)

	sendVerificationEmail(req.Email, req.Username, activationURL)

	response.JSON(w, http.StatusCreated, customerToken, false, "register successfully")
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

func (app *application) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the token from the query parameters
	token := r.URL.Query().Get("token")
	userIdStr := r.URL.Query().Get("user_id")
	if token == "" {
		app.badRequest(w, r, errors.New("missing token"))
		return
	}

	if userIdStr == "" {
		app.badRequest(w, r, errors.New("missing user_id"))
		return
	}

	userIdInt, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		app.badRequest(w, r, errors.New("invalid user_id"))
		return
	}

	// Verify the token
	storedToken, expiresAt, err := app.store.Users.GetEmailVerificationToken(ctx, userIdInt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Println("no email verification token found for user", userIdInt)
			app.badRequest(w, r, errors.New("invalid or expired token"))
			return
		}
		app.serverError(w, r, err)
		return
	}

	if storedToken != token || time.Now().After(expiresAt) {
		log.Println("stoked token is equal to token", storedToken == token, "and now is after expires", time.Now().After(expiresAt))
		log.Println("storedToken", storedToken, "token", token, "expiresAt", expiresAt, "time.Now()", time.Now())
		app.badRequest(w, r, errors.New("invalid or expired token"))
		return
	}

	if storedToken != token {
		app.badRequest(w, r, errors.New("invalid token"))
		return
	}

	// Update the user's email verification status
	err = app.store.Users.VerifyUserEmail(ctx, userIdInt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, nil, false, "email verified successfully")
}

// func (app *application) resendEmailVerificationHandler(w http.ResponseWriter, r *http.Request) {
// 	ctx := r.Context()
//
// 	// Get the user ID from the query parameters
// 	userIdStr := r.URL.Query().Get("user_id")
// 	if userIdStr == "" {
// 		app.badRequest(w, r, errors.New("missing user_id"))
// 		return
// 	}
//
// 	userIdInt, err := strconv.ParseInt(userIdStr, 10, 64)
// 	if err != nil {
// 		app.badRequest(w, r, errors.New("invalid user_id"))
// 		return
// 	}
//
// 	// Get the user by ID
// 	user, err := app.store.Users.GetById(ctx, userIdInt)
// 	if err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			app.notFound(w, r)
// 			return
// 		}
// 		app.serverError(w, r, err)
// 		return
// 	}
//
// 	if user.EmailVerified {
// 		app.badRequest(w, r, errors.New("email already verified"))
// 		return
// 	}
//
// 	// Create a new email verification token
// 	emailToken, err := app.tokenMaker.CreateRandomToken()
// 	if err != nil {
// 		app.serverError(w, r, err)
// 		return
// 	}
//
// 	expiresAt := time.Now().Add(24 * time.Hour) // Token valid for 24 hours
//
// 	err = app.store.Users.AddEmailVerificationToken(ctx, user.Id, emailToken, expiresAt)
// 	if err != nil {
// 		app.serverError(w, r, err)
// 		return
// 	}
//
// 	log.Println("Email verification token created for user", user.Username)
//
// 	activationURL := app.config.frontEndUrl + "/auth/confirm-email?token=" + emailToken + "&user_id=" + userIdStr
// 	err = sendVerificationEmail(user.Email, user.Username, activationURL)
// 	if err != nil {
// 		app.serverError(w, r, err)
// 		return
// 	}
//
// 	response.JSON(w, http.StatusOK, nil, false, "verification email resent successfully")
// }

func (app *application) validRegisterUserReq(ctx context.Context, req auth.RegisterUserReq) (exists bool, msg string, err error) {
	con1, err := app.store.Users.EmailExists(ctx, req.Email)
	// case exists
	if con1 {
		msg += "email already exists\n"
	}
	if err != nil {
		return false, msg, err
	}

	con2, err := app.store.Users.UsernameExists(ctx, req.Username)
	// case exists
	if con2 {
		msg += "username already exists\n"
	}
	if err != nil {
		return false, msg, err
	}

	return (!con1 || !con2), msg, nil
}

func sendVerificationEmail(recipient, username, activationURL string) error {
	// create email job
	data := struct {
		Username      string
		ActivationURL string
	}{
		Username:      username,
		ActivationURL: activationURL,
	}

	log.Println("sent email job with data", data)
	JobQueue <- EmailJob{
		Recipient: recipient,
		Data:      data,
		Patterns:  []string{"user_welcome.tmpl"},
	}

	return nil
}
