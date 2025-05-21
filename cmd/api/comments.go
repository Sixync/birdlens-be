package main

import (
	"log"
	"net/http"

	"github.com/sixync/birdlens-be/internal/response"
)

type CommentResponse struct {
	Id            int64   `json:"id"`
	PostId        int64   `json:"post_id"`
	UserFullName  string  `json:"user_full_name"`
	UserAvatarUrl *string `json:"user_avatar_url"`
	Content       string  `json:"content"`
	CreatedAt     string  `json:"created_at"`
}

func (app *application) getPostCommentsHandler(w http.ResponseWriter, r *http.Request) {
	limit, offset := getPaginateFromCtx(r)
	post := app.getPostFromCtx(r)
	if post == nil {
		log.Println("post is nil")
		app.notFound(w, r)
		return
	}

	log.Println("post is", post)

	ctx := r.Context()
	comments, err := app.store.Comments.GetByPostId(ctx, post.Id, limit, offset)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, comments, false, "get successful")
}
