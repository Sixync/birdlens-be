package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
)

var PostKey key = "post"

type PostResponse struct {
	PosterAvatarUrl string    `json:"poster_avatar_url"`
	PosterName      string    `json:"poster_name"`
	CreatedAt       time.Time `json:"created_at"`
	ImagesUrls      []string  `json:"images_urls"`
	Content         string    `json:"content"`
	LikesCount      int       `json:"likes_count"`
	CommentsCount   int       `json:"comments_count"`
	SharesCount     int       `json:"shares_count"`
	IsLiked         bool      `json:"is_liked"`
}

func (app *application) getPostsHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	ctx := r.Context()

	limit, offset := getPaginateFromCtx(r)
	posts, err := app.store.Posts.GetAll(ctx, limit, offset)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	var postResponses []PostResponse

	for _, post := range posts.Items {
		var postResponse PostResponse
		postResponse.PosterAvatarUrl = *currentUser.AvatarUrl
		postResponse.PosterName = currentUser.Username
		postResponse.CreatedAt = post.CreatedAt
		postResponse.Content = post.Content
		likes, err := app.store.Posts.GetLikeCounts(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		postResponse.LikesCount = likes
		comments, err := app.store.Posts.GetCommentCounts(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		postResponse.CommentsCount = comments
		imageUrls, err := app.store.Posts.GetMediaUrlsById(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		postResponse.ImagesUrls = imageUrls
		isLiked, err := app.store.Posts.UserLiked(ctx, currentUser.Id, post.Id)
		if err != nil {
			app.serverError(w, r, err)
		}
		postResponse.IsLiked = isLiked
		postResponses = append(postResponses, postResponse)
	}

	// transform to postResponse

	response.JSON(w, http.StatusOK, postResponses, false, "get successful")
}

func (app *application) addUserReactionHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	post := app.getPostFromCtx(r)
	if post == nil {
		app.badRequest(w, r, errors.New("post not found"))
		return
	}

	reactionType := r.URL.Query().Get("reaction_type")
	if reactionType == "" {
		app.badRequest(w, r, errors.New("reaction_type is required"))
		return
	}

	err := app.store.Posts.AddUserReaction(r.Context(), currentUser.Id, post.Id, reactionType)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, nil, false, "reaction added successfully")
}

type CreatePostRequest struct {
	Content string `json:"content"`
}

func (app *application) createPostHandler(w http.ResponseWriter, r *http.Request) {
	var post store.Post

	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	post.Content = r.FormValue("content")

	var uploadedFiles []uploadedFile

	for _, fheaders := range r.MultipartForm.File {
		for _, headers := range fheaders {
			// process each files
			var uploadedFile uploadedFile

			file, err := headers.Open()
			if err != nil {
				log.Println("error opening file at posts.go:", err)
				app.serverError(w, r, fmt.Errorf("failed to open file: %w", err))
				return
			}

			defer file.Close()

			// detect contentType

			buff := make([]byte, 512)

			file.Read(buff)

			file.Seek(0, 0)

			contentType := http.DetectContentType(buff)
			if contentType != "image/jpeg" && contentType != "image/png" && contentType != "video/mp4" {
				app.badRequest(w, r, fmt.Errorf("unsupported file type: %s", contentType))
				return
			}

			uploadedFile.ContentType = contentType
			log.Println("detected content type", uploadedFile.ContentType)

			// get file size

			var sizeBuff bytes.Buffer
			fileSize, err := sizeBuff.ReadFrom(file)
			if err != nil {
				log.Println("error reading file size:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file size: %w", err))
				return
			}

			file.Seek(0, 0)

			uploadedFile.Size = fileSize
			log.Println("file size", uploadedFile.Size)

			uploadedFile.Filename = headers.Filename
			log.Println("file name", uploadedFile.Filename)

			contentBuf := bytes.NewBuffer(nil)

			if _, err := io.Copy(contentBuf, file); err != nil {
				log.Println("error reading file content:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file content: %w", err))
				return
			}

			uploadedFile.FileContent = contentBuf.Bytes()
			uploadedFiles = append(uploadedFiles, uploadedFile)
		}
	}

	ctx := r.Context()

	var urls []string
	for _, uploadedFile := range uploadedFiles {
		filePath := fmt.Sprintf("posts/%v/%v", post.Id, "media")
		url, err := app.uploadFileToCloudinary(ctx, "media", post.Id, filePath, uploadedFile.FileContent)
		if err != nil {
			log.Println("error uploading post media:", err)
			app.serverError(w, r, fmt.Errorf("failed to upload post media: %w", err))
			return
		}

		err = app.store.Posts.AddMediaUrl(ctx, post.Id, url)
		if err != nil {
			log.Println("error adding media url to post:", err)
			app.serverError(w, r, fmt.Errorf("failed to add media url to post: %w", err))
			return
		}
	}

	log.Println("uploaded media urls", urls)

	post.LocationName = r.FormValue("location_name")
	post.Latitude, _ = strconv.ParseFloat(r.FormValue("latitude"), 64)
	post.Longitude, _ = strconv.ParseFloat(r.FormValue("longitude"), 64)
	post.PrivacyLevel = r.FormValue("privacy_level")
	post.Type = r.FormValue("type")
	post.IsFeatured = r.FormValue("is_featured") == "true"

	log.Println("post is", post)

	err := app.store.Posts.Create(r.Context(), &post)
	if err != nil {
		log.Println("error creating post:", err)
		app.serverError(w, r, fmt.Errorf("failed to create post: %w", err))
		return
	}

	response.JSON(w, http.StatusCreated, post, false, "post created successfully")
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
