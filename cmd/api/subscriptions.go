package main

import (
	"net/http"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/validator"
)

func (app *application) getSubscriptionsHandler(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := app.store.Subscriptions.GetAll(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusOK, subscriptions, false, "get subscriptions successful"); err != nil {
		app.serverError(w, r, err)
	}
}

type CreateSubscriptionRequest struct {
	Name         string  `json:"name" validate:"required"`
	Description  string  `json:"description" validate:"required"`
	Price        float64 `json:"price" validate:"required"`
	DurationDays int     `json:"duration_days" validate:"required"`
}

func (app *application) createSubscriptionHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateSubscriptionRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	subscription := &store.Subscription{
		Name:         req.Name,
		Description:  req.Description,
		Price:        req.Price,
		DurationDays: req.DurationDays,
	}

	if err := app.store.Subscriptions.Create(r.Context(), subscription); err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusCreated, subscription, false, "subscription created successfully"); err != nil {
		app.serverError(w, r, err)
	}
}
