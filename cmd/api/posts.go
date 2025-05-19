package main

import (
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

func (app *application) getPostsHandler(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPaginateFromCtx(r)
	ctx := r.Context()
	posts, err := app.store.Posts.GetAll(ctx, limit, offset)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, posts, false, "get successful")
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
