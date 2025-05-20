package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/sixync/birdlens-be/internal/response"
)

func (app *application) getUserFollowersHandler(w http.ResponseWriter, r *http.Request) {
	userId := r.PathValue("user_id")
	log.Println(userId)
	userIdInt, err := strconv.ParseInt(userId, 10, 64)
	log.Println(userIdInt)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	ctx := r.Context()
	result, err := app.store.Followers.GetByUserId(ctx, userIdInt)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := response.JSON(w, http.StatusOK, result, false, "get successful"); err != nil {
		app.serverError(w, r, err)
	}
}
