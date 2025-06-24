// birdlens-be/cmd/api/auth.go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sixync/birdlens-be/auth"
	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/utils"
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
	app.logger.Info("<<<<< LOGIN HANDLER REACHED >>>>>", "method", r.Method, "path", r.URL.Path)
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
		app.logger.Error("Failed to get user subscription by email during login", "email", req.Email, "error", err)
		app.serverError(w, r, err)
		return
	}
	if err == nil && subscription != nil {
		app.logger.Info("User subscription found during login", "email", req.Email, "subscription_name", subscription.Name)
		subParam = subscription.Name
	} else {
		app.logger.Info("No active subscription found or user does not exist for subscription check during login", "email", req.Email)
	}

	customToken, err := app.authService.Login(ctx, req.Email, req.Password, subParam)
	if err != nil {
		app.logger.Info("AuthService.Login returned an error", "email", req.Email, "error_type", fmt.Sprintf("%T", err), "error_value", err.Error())
		switch {
		case errors.Is(err, auth.ErrUserNotFound):
			app.logger.Warn("Login attempt failed: User not found", "email", req.Email)
			app.invalidCredentials(w, r) // Returns 400 "invalid credentials"
		case errors.Is(err, auth.ErrIncorrectPassword):
			app.logger.Warn("Login attempt failed: Incorrect password", "email", req.Email)
			app.invalidCredentials(w, r) // Returns 400 "invalid credentials"
		case errors.Is(err, auth.ErrMailNotVerified):
			app.logger.Info("Login attempt failed: Email not verified", "email", req.Email)
			app.badRequest(w, r, errors.New("email not verified, please check your inbox for a verification link")) // Returns 400
		default:
			app.logger.Error("Unhandled error during login process", "email", req.Email, "error", err)
			app.serverError(w, r, err) // Returns 500
		}
		return
	}

	app.logger.Info("Login successful, custom token generated", "email", req.Email)
	response.JSON(w, http.StatusOK, customToken, false, "login successful")
}

func (app *application) registerHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("<<<<< REGISTER HANDLER REACHED >>>>>", "method", r.Method, "path", r.URL.Path)
	var req auth.RegisterUserReq
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

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
		app.logger.Warn("Invalid registration request", "reason", msg, "email", req.Email, "username", req.Username)
		app.badRequest(w, r, errors.New(msg))
		return
	}

	app.logger.Info("Valid registration request", "email", req.Email, "username", req.Username)

	customerToken, userId, err := app.authService.Register(ctx, req)
	if err != nil {
		app.logger.Error("Error during user registration in auth service", "email", req.Email, "error", err)
		app.badRequest(w, r, err)
		return
	}

	err = app.store.Roles.AddUserToRole(ctx, userId, store.USER)
	if err != nil {
		app.logger.Error("Error adding user to role during registration", "user_id", userId, "error", err)
		app.serverError(w, r, err)
		return
	}

	emailToken, err := app.tokenMaker.CreateRandomToken()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	duration := time.Duration(app.config.emailVerificationExpiresInHours) * time.Hour
	expiresAt := time.Now().Add(duration)

	app.store.Users.AddEmailVerificationToken(ctx, userId, emailToken, expiresAt)

	links := url.Values{
		"token":   []string{emailToken},
		"user_id": []string{strconv.FormatInt(userId, 10)},
	}
	// Logic: Use the frontend URL for email verification links, not the backend base URL.
	// This ensures users are directed to the Android app or web frontend.
	// The frontend will then call the backend API at /auth/verify-email.
	activationURL := app.config.frontEndUrl + "/auth/verify-email?" + links.Encode()

	app.logger.Info("Activation URL generated for user", "user_id", userId, "url_fragment", "/auth/verify-email?"+links.Encode())

	sendVerificationEmail(req.Email, req.Username, activationURL)

	response.JSON(w, http.StatusCreated, customerToken, false, "register successfully, please check your email to verify your account")
}

func (app *application) refreshTokenHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("<<<<< REFRESH TOKEN HANDLER REACHED >>>>>", "method", r.Method, "path", r.URL.Path)
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
		app.logger.Warn("Error getting user claims from token during refresh", "error", err)
		app.unauthorized(w, r)
		return
	}

	if claims == nil {
		app.logger.Warn("Claims are nil during token refresh")
		app.unauthorized(w, r)
		return
	}

	session, err := app.store.Sessions.GetById(ctx, claims.ID)
	if err != nil {
		app.logger.Warn("Session not found during token refresh", "session_id", claims.ID, "error", err)
		app.unauthorized(w, r)
		return
	}

	if session.IsRevoked || session.ExpiresAt.Before(time.Now()) {
		app.logger.Warn("Session is revoked or expired during token refresh", "session_id", claims.ID, "is_revoked", session.IsRevoked, "expires_at", session.ExpiresAt)
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
	app.logger.Info("Handling user session", "user_id", user.Id, "email", user.Email)
	s, err := app.store.Sessions.GetById(ctx, user.Id)

	if err != nil && errors.Is(err, sql.ErrNoRows) {
		app.logger.Info("Creating new session for user", "user_id", user.Id)
		session := &store.Session{
			ID:           user.Id,
			UserEmail:    user.Email,
			RefreshToken: refreshToken,
			IsRevoked:    false,
			ExpiresAt:    refreshTokenExp,
		}
		err = app.store.Sessions.Create(ctx, session)
		if err != nil {
			app.logger.Error("Failed to create new session", "user_id", user.Id, "error", err)
			return err
		}
	} else if err == nil {
		app.logger.Info("Updating existing session for user", "user_id", user.Id, "old_expires_at", s.ExpiresAt, "new_expires_at", refreshTokenExp)
		s.ExpiresAt = refreshTokenExp
		s.RefreshToken = refreshToken
		err := app.store.Sessions.UpdateSession(ctx, s)
		if err != nil {
			app.logger.Error("Failed to update session", "user_id", user.Id, "error", err)
			return err
		}
	} else {
		app.logger.Error("Error fetching session for user", "user_id", user.Id, "error", err)
		return err
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

func (app *application) verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("<<<<< VERIFY EMAIL HANDLER REACHED >>>>>", "method", r.Method, "path", r.URL.Path, "query", r.URL.RawQuery)
	ctx := r.Context()

	token := r.URL.Query().Get("token")
	userIdStr := r.URL.Query().Get("user_id")

	if token == "" || userIdStr == "" {
		app.logger.Warn("verifyEmailHandler: missing token or user_id", "token_present", token != "", "user_id_present", userIdStr != "")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, "Missing token or user ID in verification link.")
		return
	}

	userIdInt, err := strconv.ParseInt(userIdStr, 10, 64)
	if err != nil {
		app.logger.Warn("verifyEmailHandler: invalid user_id format", "user_id_str", userIdStr, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, "Invalid user ID format.")
		return
	}

	storedToken, expiresAt, err := app.store.Users.GetEmailVerificationToken(ctx, userIdInt)
	if err != nil {
		errMsg := "Invalid or expired verification link."
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Info("No email verification token found for user", "user_id", userIdInt)
		} else {
			app.logger.Error("Error getting email verification token from store", "user_id", userIdInt, "error", err)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, errMsg)
		return
	}

	if storedToken != token || time.Now().After(expiresAt) {
		app.logger.Warn("verifyEmailHandler: token mismatch or expired",
			"user_id", userIdInt,
			"token_matches", storedToken == token,
			"is_expired", time.Now().After(expiresAt),
			"expires_at", expiresAt)
		errMsg := "Verification link is invalid or has expired."
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, verificationFailedHTML, errMsg)
		return
	}

	err = app.store.Users.VerifyUserEmail(ctx, userIdInt)
	if err != nil {
		app.logger.Error("Error updating user email verification status", "user_id", userIdInt, "error", err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, verificationFailedHTML, "An error occurred while verifying your email. Please try again.")
		return
	}

	app.logger.Info("Email verified successfully", "user_id", userIdInt)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, verificationSuccessHTML)
}

type ForgotPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func (app *application) forgotPasswordHandler(w http.ResponseWriter, r *http.Request) {
	app.logger.Info("<<<<< FORGOT PASSWORD HANDLER REACHED >>>>>", "method", r.Method, "path", r.URL.Path)
	var req ForgotPasswordRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	token, err := app.tokenMaker.CreateRandomToken()
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	app.logger.Info("Generated reset password token", "email", req.Email, "token", token)

	duration := time.Duration(app.config.forgotPasswordExpiresInHours) * time.Hour
	expires := time.Now().Add(duration)
	if expires.Before(time.Now()) {
		app.logger.Debug("Reset password token expiration time is in the past with expires", "expires_at", expires, "duration", duration, "current_time", time.Now())
	}

	app.logger.Info("Reset password token will expire at", "expires_at", expires)

	ctx := r.Context()

	err = app.store.Users.AddResetPasswordToken(ctx, req.Email, token, expires)
	if err != nil {
		app.logger.Error("Error adding reset password token", "email", req.Email, "error", err)
		app.serverError(w, r, err)
		return
	}

	links := url.Values{
		"token": []string{token},
	}
	// Use the frontend URL for the reset password link
	resetUrl := app.config.frontEndUrl + "/reset-password?" + links.Encode()
	app.logger.Info("Reset password URL generated", "email", req.Email, "reset_url", resetUrl)

	linkValidity := app.config.forgotPasswordExpiresInHours

	sendResetPasswordEmail(req.Email, resetUrl, linkValidity)

	app.logger.Info("Reset password email sent", "email", req.Email, "reset_url", resetUrl)
	response.JSON(w, http.StatusOK, nil, false, "reset password email sent successfully")
}

type ResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=3"`
}

func (app *application) resetPasswordHandler(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()
	user, err := app.store.Users.GetUserByResetPasswordToken(ctx, req.Token)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.logger.Warn("Reset password token not found", "token", req.Token)
			app.badRequest(w, r, errors.New("invalid or expired reset password token"))
		default:
			app.logger.Error("Error retrieving user by reset password", "token", req.Token, "error", err)
			app.serverError(w, r, err)
		}
		return
	}

	newHashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		app.logger.Error("Error hashing new password", "error", err)
		app.serverError(w, r, err)
		return
	}

	user.HashedPassword = &newHashedPassword
	if err := app.store.Users.Update(ctx, user); err != nil {
		app.logger.Error("Error updating user password", "user_id", user.Id, "error", err)
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("User password reset successfully", "user_id", user.Id)
	response.JSON(w, http.StatusOK, nil, false, "password reset successfully")
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

	return (!con1 && !con2), msg, nil
}

func sendVerificationEmail(recipient, username, activationURL string) error {
	data := struct {
		Username      string
		ActivationURL string
	}{
		Username:      username,
		ActivationURL: activationURL,
	}

	slog.Info("Queuing verification email job", "recipient", recipient, "username", username)
	JobQueue <- EmailJob{
		Recipient: recipient,
		Data:      data,
		Patterns:  []string{"user_welcome.tmpl"},
	}

	return nil
}

func sendResetPasswordEmail(recipient, resetURL string, linkValidity int) error {
	data := struct {
		ResetLink    string
		LinkValidity int
	}{
		ResetLink:    resetURL,
		LinkValidity: linkValidity,
	}

	slog.Info("Queuing reset password email job", "recipient", recipient)
	JobQueue <- EmailJob{
		Recipient: recipient,
		Data:      data,
		Patterns:  []string{"reset_password.tmpl"},
	}

	return nil
}

var htmlTmpl = template.Must(template.New("messagePage").Parse(`
<!DOCTYPE html><html><head><title>{{.Title}}</title></head><body><h1>{{.Title}}</h1><p>{{.Message}}</p></body></html>`))

type HtmlPageData struct {
	Title   string
	Message string
}

