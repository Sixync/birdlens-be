// birdlens-be/cmd/api/auth.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
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
			app.badRequest(w, r, errors.New("email not verified, please check your inbox for a verification link"))
		default:
			log.Println("error logging in user with email", req.Email, "and error", err)
			app.serverError(w, r, err)
		}
		return
	}

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
	// Use app.config.baseURL which should be the public URL of the backend (e.g., http://localhost)
	// The path will be /auth/verify-email as defined in routes.go
	activationURL := app.config.baseURL + "/auth/verify-email?" + links.Encode()

	log.Println("activationURL", activationURL)

	sendVerificationEmail(req.Email, req.Username, activationURL)

	response.JSON(w, http.StatusCreated, customerToken, false, "register successfully, please check your email to verify your account")
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

const verificationSuccessHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Email Verified</title>
    <style>
        body { font-family: Arial, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background-color: #f4f4f4; text-align: center; }
        .container { padding: 20px; background-color: white; border-radius: 8px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        h1 { color: #4CAF50; }
        p { color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Email Verified Successfully!</h1>
        <p>Your email address has been successfully verified. You can now close this window and log in to the Birdlens app.</p>
    </div>
</body>
</html>
`

const verificationFailedHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Verification Failed</title>
    <style>
        body { font-family: Arial, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background-color: #f4f4f4; text-align: center; }
        .container { padding: 20px; background-color: white; border-radius: 8px; box-shadow: 0 0 10px rgba(0,0,0,0.1); }
        h1 { color: #F44336; }
        p { color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Email Verification Failed</h1>
        <p>%s</p>
        <p>Please try registering again or contact support if the issue persists.</p>
    </div>
</body>
</html>
`

// verifyEmailHandler will now be a GET request and respond with HTML.
func (app *application) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get the token from the query parameters
	token := r.URL.Query().Get("token")
	userIdStr := r.URL.Query().Get("user_id")

	if token == "" || userIdStr == "" {
		log.Println("verifyEmailHandler: missing token or user_id")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, "Missing token or user ID in verification link.")
		return
	}

	userIdInt, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		log.Printf("verifyEmailHandler: invalid user_id format: %s", userIdStr)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, "Invalid user ID format.")
		return
	}

	// Verify the token
	storedToken, expiresAt, err := app.store.Users.GetEmailVerificationToken(ctx, userIdInt)
	if err != nil {
		log.Printf("verifyEmailHandler: error getting token for user %d: %v", userIdInt, err)
		errMsg := "Invalid or expired verification link."
		if errors.Is(err, sql.ErrNoRows) {
			log.Println("no email verification token found for user", userIdInt)
		} else {
			// For other DB errors, don't expose details to user
			// app.serverError(w, r, err) // This sends JSON, we want HTML
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest) // Or StatusNotFound
		fmt.Fprintf(w, verificationFailedHTML, errMsg)
		return
	}

	if storedToken != token || time.Now().After(expiresAt) {
		log.Printf("verifyEmailHandler: token mismatch or expired. Stored: %s, Received: %s, Expires: %v, Now: %v", storedToken, token, expiresAt, time.Now())
		errMsg := "Verification link is invalid or has expired."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, errMsg)
		return
	}

	// Update the user's email verification status
	err = app.store.Users.VerifyUserEmail(ctx, userIdInt)
	if err != nil {
		log.Printf("verifyEmailHandler: error updating user email verification status for user %d: %v", userIdInt, err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, verificationFailedHTML, "An error occurred while verifying your email. Please try again.")
		return
	}

	log.Printf("verifyEmailHandler: email verified successfully for user %d", userIdInt)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, verificationSuccessHTML)
}


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

	return (!con1 && !con2), msg, nil
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

// Template for parsing HTML responses directly from handler
var htmlTmpl = template.Must(template.New("messagePage").Parse(`
<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body><h1>{{.Title}}</h1><p>{{.Message}}</p></body></html>`))

type HtmlPageData struct {
	Title   string
	Message string
}