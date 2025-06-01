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
		r.With(app.paginate).With(app.getPostMiddleware).Get("/{post_id}/comments", app.getPostCommentsHandler)
	})

	mux.Route("/users", func(r chi.Router) {
		r.With(app.paginate).With(app.getUserMiddleware).Get("/{user_id}/followers", app.getUserFollowersHandler)
		r.Post("/", app.createUserHandler)
		r.With(app.AuthMiddleware).Get("/me", app.getCurrentUserProfileHandler)
	})

	mux.Route("/auth", func(r chi.Router) {
		r.Post("/login", app.loginHandler)
		r.Post("/register", app.registerHandler)
		r.Post("/refresh_token", app.refreshTokenHandler)
	})

	mux.Route("/tours", func(r chi.Router) {
		// TODO: Add authentication middleware later
		r.With(app.paginate).Get("/", app.getToursHandler)
		r.With(app.getTourMiddleware).Get("/{tour_id}", app.getTourHandler)
		r.Post("/", app.createTourHandler)
		r.With(app.getTourMiddleware).Put("/{tour_id}/images", app.addTourImagesHandler)
		r.With(app.getTourMiddleware).Put("/{tour_id}/thumbnail", app.addTourThumbnailHandler)
	})

	// subscriptions
	mux.Route("/subscriptions", func(r chi.Router) {
		// TODO: Add authentication middleware later
		r.Get("/", app.getSubscriptionsHandler)
		r.Post("/", app.createSubscriptionHandler)
		// TODO: This one
		// r.Put("/{user_id}/subscribe", app.subscribeUserHandler)
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
