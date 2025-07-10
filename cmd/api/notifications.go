package main

import (
	"log/slog"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

func (app *application) getNotificationsHandler(w http.ResponseWriter, r *http.Request) {
	user := app.getUserFromFirebaseClaimsCtx(r)
	if user == nil {
		app.unauthorized(w, r)
		return
	}

	limit, offset := getPaginateFromCtx(r)

	notifications, err := app.store.Notifications.GetByUserID(r.Context(), user.Id, limit, offset)
	if err != nil {
		slog.Error("Failed to get notifications for user", "user_id", user.Id, "error", err)
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, notifications, false, "Notifications retrieved successfully")
}