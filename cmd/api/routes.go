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

	mux.Group(func(mux chi.Router) {
		mux.Use(app.requireBasicAuthentication)
	})

	mux.Route("/posts", func(r chi.Router) {
		r.With(app.paginate).Get("/", app.getPostsHandler)
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
