package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

var PostKey key = "post"

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

func (app *application) getPostMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postId := r.PathValue("post_id")
		if postId == "" {
			app.badRequest(w, r, errors.New("post_id is required"))
			return
		}

		postIdInt, err := strconv.ParseInt(postId, 10, 64)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		post, err := app.store.Posts.GetById(r.Context(), postIdInt)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		log.Println("post from middlware", post)

		ctx := context.WithValue(r.Context(), PostKey, post)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) getPostFromCtx(r *http.Request) *store.Post {
	ctx := r.Context()
	post, ok := ctx.Value(PostKey).(*store.Post)
	log.Println("post from ctx", post)
	if !ok {
		return nil
	}
	return post
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
