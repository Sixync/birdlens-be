package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"strings"

	"github.com/sixync/birdlens-be/internal/jwt"
	"github.com/sixync/birdlens-be/internal/response"

	"github.com/tomasen/realip"
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

func getPaginateFromCtx(r *http.Request) (limit, offset int) {
	ctx := r.Context()
	limit, ok := ctx.Value(LimitKey).(int)
	if !ok {
		limit = 10
	}
	offset, ok = ctx.Value(OffsetKey).(int)
	if !ok {
		offset = 0
	}
	return
}

func (app *application) authMiddleware(next http.Handler) http.Handler {
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
		token, err := app.authService.FireAuth.VerifyIDToken(r.Context(), tokenStr)
		if err != nil {
			log.Println("Failed to verify ID token:", err)
			app.errorMessage(w, r, http.StatusUnauthorized, "Invalid or expired token", nil)
			return
		}

		if token == nil {
			log.Println("token is null")
			app.errorMessage(w, r, http.StatusUnauthorized, "Invalid or expired token", nil)
			return
		}

		claims := jwt.NewFirebaseClaims(token)

		// Token is valid, store claims in context
		ctx := context.WithValue(r.Context(), UserClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) adminRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value(UserClaimsKey).(*jwt.FirebaseClaims)
		if !ok || claims == nil {
			app.unauthorized(w, r)
			return
		}

		if !claims.() {
			app.forbidden(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}
