package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

var EventKey key = "event"

type CreateEventRequest struct {
	Title       string     `json:"title" validate:"required,min=1,max=100"`
	Description string     `json:"description" validate:"required,min=1,max=1000"`
	StartDate   string     `json:"start_date" validate:"required"`
	EndDate     string     `json:"end_date" validate:"required"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   *time.Time `json:"updated_at"`
}

func (app *application) createEventHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateEventRequest

	ctx := r.Context()

	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		app.badRequest(w, r, fmt.Errorf("invalid start_date: %v", err))
		return
	}

	// Parse the end_date into time.Time
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		app.badRequest(w, r, fmt.Errorf("invalid end_date: %v", err))
		return
	}
	event := &store.Event{
		Description: req.Description,
		StartDate:   startDate,
		EndDate:     endDate,
	}

	err = app.store.Events.Create(ctx, event)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, event, false, "event created successfully")
}

func (app *application) getEventsHandler(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPaginateFromCtx(r)

	events, err := app.store.Events.GetAll(r.Context(), limit, offset)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, events, false, "events retrieved successfully")
}

func (app *application) getEventHandler(w http.ResponseWriter, r *http.Request) {
	event, err := getEventFromCtx(r)
	log.Println("getEventHandler event", event)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if event == nil {
		app.notFound(w, r)
		return
	}
	response.JSON(w, http.StatusOK, event, false, "event retrieved successfully")
}

func (app *application) deleteEventHandler(w http.ResponseWriter, r *http.Request) {
	event, err := getEventFromCtx(r)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if event == nil {
		app.notFound(w, r)
		return
	}

	err = app.store.Events.Delete(r.Context(), event.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, nil, false, "event deleted successfully")
}

func getEventFromCtx(r *http.Request) (*store.Event, error) {
	ctx := r.Context()
	event, _ := ctx.Value(EventKey).(*store.Event)
	log.Println("getEventFromCtx event", event)
	return event, nil
}

func (app *application) getEventMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventId := r.PathValue("event_id")
		if eventId == "" {
			app.badRequest(w, r, errors.New("event_id is required"))
			return
		}

		eventIdInt, err := strconv.ParseInt(eventId, 10, 64)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		event, err := app.store.Events.GetByID(r.Context(), eventIdInt)
		log.Println("getEventMiddleware event", event)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			app.notFound(w, r)
			return
		case err != nil:
			app.serverError(w, r, err)
			return
		}

		ctx := context.WithValue(r.Context(), EventKey, event)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
