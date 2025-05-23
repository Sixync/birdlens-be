package main

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

var TourKey key = "tour"

func (app *application) getToursHandler(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPaginateFromCtx(r)
	ctx := r.Context()
	tours, err := app.store.Tours.GetAll(ctx, limit, offset)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, tours, false, "get tours successfully")
}

func (app *application) getTourHandler(w http.ResponseWriter, r *http.Request) {
	tour, err := app.getTourFromContext(r)

	ctx := r.Context()
	event, err := app.store.Events.GetByID(ctx, tour.EventId)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	tour.Event = event

	location, err := app.store.Location.GetByID(ctx, tour.LocationId)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	tour.Location = location

	response.JSON(w, http.StatusOK, tour, false, "get tour successfully")
}

// get tour middleware
func (app *application) getTourMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tourIdStr := r.PathValue("tour_id")
		if tourIdStr == "" {
			app.badRequest(w, r, errors.New("tour_id is required"))
			return
		}

		tourId, err := strconv.ParseInt(tourIdStr, 10, 64)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		tour, err := app.store.Tours.GetByID(r.Context(), tourId)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		if tour == nil {
			app.notFound(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), TourKey, tour)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) getTourFromContext(r *http.Request) (*store.Tour, error) {
	tour, ok := r.Context().Value(TourKey).(*store.Tour)
	if !ok {
		return nil, errors.New("tour not found in context")
	}

	return tour, nil
}
