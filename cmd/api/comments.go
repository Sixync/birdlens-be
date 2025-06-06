package main

import (
	"errors"
	"log"
	"net/http"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
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

// TODO: Add media to comment
type CreateCommentRequest struct {
	Content string `json:"content"`
}

func (app *application) createCommentHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	var req CreateCommentRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	log.Println("create comment request", req)

	post := app.getPostFromCtx(r)
	if post == nil {
		app.badRequest(w, r, errors.New("post not found"))
		return
	}

	var comment store.Comment
	comment.PostID = post.Id
	comment.Content = req.Content
	comment.UserID = currentUser.Id

	err := app.store.Comments.Create(r.Context(), &comment)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, comment, false, "comment created successfully")
}
