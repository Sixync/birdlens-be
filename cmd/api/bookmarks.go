package main

import (
	"fmt"
	"net/http"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

type CreateBookmarkRequest struct {
	HotspotLocationID string `json:"hotspot_location_id" validate:"required"`
}

func (app *application) CreateBookmarkHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := getUserFromCtx(r)
	if err != nil {
		app.unauthorized(w, r)
	}

	var req CreateBookmarkRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return

	}
	// check if user already has a bookmark for this hotspot location
	exists, err := app.store.Bookmarks.Exists(ctx, user.Id, req.HotspotLocationID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if exists {
		app.badRequest(w, r, fmt.Errorf("user already has a bookmark for this hotspot location"))
		return
	}

	bookmark := req.toBookmark(user.Id)

	if err := app.store.Bookmarks.Create(ctx, bookmark); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	response.JSON(w, http.StatusCreated, bookmark, false, "Bookmark created successfully")
}

func (app *application) GetByUserIdHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := getUserFromCtx(r)
	if err != nil {
		app.unauthorized(w, r)
		return
	}

	bookmarks, err := app.store.Bookmarks.GetByUserID(ctx, user.Id)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, bookmarks, false, "Bookmarks retrieved successfully")
}

func (req *CreateBookmarkRequest) toBookmark(userID int64) *store.Bookmark {
	return &store.Bookmark{
		UserID:            userID,
		HotspotLocationId: req.HotspotLocationID,
	}
}
