package main

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {
	mux := chi.NewRouter()

	mux.NotFound(app.notFound)
	mux.MethodNotAllowed(app.methodNotAllowed)

	mux.Use(app.logAccess)
	mux.Use(app.recoverPanic)

	mux.Route("/posts", func(r chi.Router) {
		r.With(app.paginate).Get("/", app.getPostsHandler)
	})

	mux.Route("/users", func(r chi.Router) {
		r.With(app.paginate).With(app.getUserMiddleware).Get("/{user_id}/followers", app.getUserFollowersHandler)
		r.Post("/", app.createUserHandler)
		r.With(app.AuthMiddleware).Get("/me", app.getCurrentUserProfileHandler)
	})

	mux.Route("/auth", func(r chi.Router) {
		r.Post("/login", app.loginHandler)
		r.Post("/register", app.registerHandler)
	})

	return mux
}

func (app *application) GetQueryInt(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	result, err := strconv.Atoi(value)
	if err != nil {
		return -1, err
	}

	return result, nil
}
