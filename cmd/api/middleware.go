package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/response"

	"github.com/tomasen/realip"
	"golang.org/x/crypto/bcrypt"
)

type key string

var (
	LimitKey      key = "limit"
	OffsetKey     key = "offset"
	UserClaimsKey key = "user_claims"
)

func (app *application) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			pv := recover()
			if pv != nil {
				app.serverError(w, r, fmt.Errorf("%v", pv))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

func (app *application) logAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mw := response.NewMetricsResponseWriter(w)
		next.ServeHTTP(mw, r)

		var (
			ip     = realip.FromRequest(r)
			method = r.Method
			url    = r.URL.String()
			proto  = r.Proto
		)

		userAttrs := slog.Group("user", "ip", ip)
		requestAttrs := slog.Group("request", "method", method, "url", url, "proto", proto)
		responseAttrs := slog.Group("repsonse", "status", mw.StatusCode, "size", mw.BytesCount)

		app.logger.Info("access", userAttrs, requestAttrs, responseAttrs)
	})
}

func (app *application) requireBasicAuthentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, plaintextPassword, ok := r.BasicAuth()
		if !ok {
			app.basicAuthenticationRequired(w, r)
			return
		}

		if app.config.basicAuth.username != username {
			app.basicAuthenticationRequired(w, r)
			return
		}

		err := bcrypt.CompareHashAndPassword([]byte(app.config.basicAuth.hashedPassword), []byte(plaintextPassword))
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			app.basicAuthenticationRequired(w, r)
			return
		case err != nil:
			app.serverError(w, r, err)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (app *application) paginate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		limit, err := app.GetQueryInt(r, "limit")
		if err != nil {
			limit = 10
		}
		offset, err := app.GetQueryInt(r, "offset")
		if err != nil {
			offset = 0
		}

		ctx = context.WithValue(ctx, LimitKey, limit)
		ctx = context.WithValue(ctx, OffsetKey, offset)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			app.logger.Warn("Missing Authorization header")
			app.unauthorized(w, r)
			return
		}

		headerParts := strings.Split(authHeader, " ")
		if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
			app.logger.Warn("Malformed Authorization header", "header", authHeader)
			app.unauthorized(w, r)
			return
		}

		tokenStr := headerParts[1]
		claims, err := app.tokenMaker.VerifyToken(tokenStr)
		if err != nil {
			app.logger.Warn("Invalid token", "error", err, "token", tokenStr)
			app.errorMessage(w, r, http.StatusUnauthorized, "Invalid or expired token", nil)
			return
		}

		// Token is valid, store claims in context
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Helper to get UserClaims from context (optional but recommended)
func (app *application) getUserClaimsFromCtx(r *http.Request) *jwt.UserClaims {
	claims, ok := r.Context().Value(UserClaimsKey).(*jwt.UserClaims)
	if !ok {
		// This should not happen if middleware is correctly applied
		// and always sets the claims. Could panic or log an error.
		app.logger.Error("User claims not found in context where expected")
		return nil
	}
	return claims
}
