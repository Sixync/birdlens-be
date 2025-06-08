package main 

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

// EnrichedCommentResponse includes user details for the comment.
// This struct is part of 'package main' as it's used by handlers in this package.
type EnrichedCommentResponse struct {
	Id            int64     `json:"id"`
	PostId        int64     `json:"post_id"`
	UserFullName  string    `json:"user_full_name"`
	UserAvatarUrl *string   `json:"user_avatar_url"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
}

// CreateCommentRequest is also part of 'package main'.
type CreateCommentRequest struct {
	Content string `json:"content"`
}

// PaginatedEnrichedCommentResponse is a type alias or a new struct if needed
// to wrap PaginatedList with EnrichedCommentResponse for clarity within the main package.
// This helps avoid direct store.PaginatedList[main.EnrichedCommentResponse] if that causes issues.
// However, usually store.PaginatedList[EnrichedCommentResponse] should work if EnrichedCommentResponse is defined in the same package.
// Let's try using store.PaginatedList directly first.

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
	paginatedStoreComments, err := app.store.Comments.GetByPostId(ctx, post.Id, limit, offset)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	var enrichedComments []EnrichedCommentResponse // This is main.EnrichedCommentResponse
	if paginatedStoreComments.Items != nil {
		for _, comment := range paginatedStoreComments.Items {
			user, err := app.store.Users.GetById(ctx, comment.UserID)
			var userFullName string
			var userAvatarUrl *string

			if err != nil {
				log.Printf("Error fetching user %d for comment %d: %v. Using placeholders.", comment.UserID, comment.ID, err)
				userFullName = "Unknown User"
				// userAvatarUrl remains nil
			} else {
				userFullName = user.FirstName + " " + user.LastName
				userAvatarUrl = user.AvatarUrl
			}

			enrichedComments = append(enrichedComments, EnrichedCommentResponse{
				Id:            comment.ID,
				PostId:        comment.PostID,
				UserFullName:  userFullName,
				UserAvatarUrl: userAvatarUrl,
				Content:       comment.Content,
				CreatedAt:     comment.CreatedAt,
			})
		}
	}

	// Create a new paginated list with the enriched comments
	// This should be store.PaginatedList[*main.EnrichedCommentResponse] effectively
	enrichedPaginatedResponse := &store.PaginatedList[EnrichedCommentResponse]{
		Items:      enrichedComments,
		TotalCount: paginatedStoreComments.TotalCount,
		Page:       paginatedStoreComments.Page,
		PageSize:   paginatedStoreComments.PageSize,
		TotalPages: paginatedStoreComments.TotalPages,
	}

	response.JSON(w, http.StatusOK, enrichedPaginatedResponse, false, "get successful")
}

func (app *application) createCommentHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	var req CreateCommentRequest // This is main.CreateCommentRequest
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

	// Enrich the response for the created comment
	enrichedComment := EnrichedCommentResponse{ // This is main.EnrichedCommentResponse
		Id:            comment.ID,
		PostId:        comment.PostID,
		UserFullName:  currentUser.FirstName + " " + currentUser.LastName,
		UserAvatarUrl: currentUser.AvatarUrl,
		Content:       comment.Content,
		CreatedAt:     comment.CreatedAt,
	}

	response.JSON(w, http.StatusCreated, enrichedComment, false, "comment created successfully")
}