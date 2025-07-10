package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	services "github.com/sixync/birdlens-be/services/posts"
)

var PostKey key = "post"

type PostResponse struct {
	ID                int64      `json:"id"`
	PosterAvatarUrl   *string    `json:"poster_avatar_url"`
	PosterName        string     `json:"poster_name"`
	CreatedAt         time.Time  `json:"created_at"`
	ImagesUrls        []string   `json:"images_urls"`
	Content           string     `json:"content"`
	LikesCount        int        `json:"likes_count"`
	CommentsCount     int        `json:"comments_count"`
	SharesCount       int        `json:"shares_count"`
	IsLiked           bool       `json:"is_liked"`
	Type              string     `json:"type"`
	SightingDate      *time.Time `json:"sighting_date,omitempty"`
	TaggedSpeciesCode *string    `json:"tagged_species_code,omitempty"`
	LocationName      string     `json:"location_name,omitempty"`
	Latitude          float64    `json:"latitude,omitempty"`
	Longitude         float64    `json:"longitude,omitempty"`
}

func (app *application) getPostsHandler(w http.ResponseWriter, r *http.Request) {
	currentUser := app.getUserFromFirebaseClaimsCtx(r)
	if currentUser == nil {
		app.unauthorized(w, r)
		return
	}

	postType := r.URL.Query().Get("type")

	postRetrievalStrategy := app.getPostRetrievalStrategy(postType)

	ctx := r.Context()

	limit, offset := getPaginateFromCtx(r)
	posts, err := postRetrievalStrategy.RetrievePosts(ctx, currentUser.Id, limit, offset)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	log.Println("posts retrieved from db", posts)

	log.Println("user from claims", currentUser)
	log.Println("posts from db", posts)

	var postResponses []PostResponse

	for _, post := range posts.Items {
		var postResponse PostResponse

		postResponse.ID = post.Id
		poster, err := app.store.Users.GetById(ctx, post.UserId)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		postResponse.PosterAvatarUrl = poster.AvatarUrl
		postResponse.PosterName = poster.Username
		postResponse.CreatedAt = post.CreatedAt
		postResponse.Content = post.Content
		postResponse.Type = post.Type
		postResponse.SightingDate = post.SightingDate
		postResponse.TaggedSpeciesCode = post.TaggedSpeciesCode
		postResponse.LocationName = post.LocationName
		postResponse.Latitude = post.Latitude
		postResponse.Longitude = post.Longitude
		likes, err := app.store.Posts.GetLikeCounts(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		log.Println("likes count for post", post.Id, "is", likes)
		postResponse.LikesCount = likes
		comments, err := app.store.Posts.GetCommentCounts(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		log.Println("comments count for post", post.Id, "is", comments)
		postResponse.CommentsCount = comments
		imageUrls, err := app.store.Posts.GetMediaUrlsById(ctx, post.Id)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		log.Println("image urls for post", post.Id, "are", imageUrls)
		postResponse.ImagesUrls = imageUrls
		isLiked, err := app.store.Posts.UserLiked(ctx, currentUser.Id, post.Id)
		if err != nil {
			app.serverError(w, r, err)
		}
		log.Println("is liked by user", currentUser.Id, "for post", post.Id, "is", isLiked)
		postResponse.IsLiked = isLiked
		postResponses = append(postResponses, postResponse)
	}
	res := store.PaginatedList[PostResponse]{
		Items:      postResponses,
		TotalCount: posts.TotalCount,
		TotalPages: posts.TotalPages,
		Page:       posts.Page,
		PageSize:   posts.PageSize,
	}

	log.Println("post responses", postResponses)

	response.JSON(w, http.StatusOK, res, false, "get successful")
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
	post.LocationName = r.FormValue("location_name")
	post.Latitude, _ = strconv.ParseFloat(r.FormValue("latitude"), 64)
	post.Longitude, _ = strconv.ParseFloat(r.FormValue("longitude"), 64)
	post.PrivacyLevel = r.FormValue("privacy_level")
	post.Type = r.FormValue("type")
	post.IsFeatured = r.FormValue("is_featured") == "true"
	post.UserId = currentUser.Id

	log.Println("post is", post)

	ctx := r.Context()
	if err := app.store.Posts.Create(ctx, &post); err != nil {
		log.Println("error creating post:", err)
		app.serverError(w, r, fmt.Errorf("failed to create post: %w", err))
		return
	}

	log.Println("post created successfully with id", post.Id)
    
	// Logic: After creating the post, we check if this action should trigger a referral reward.
	// This is done in a background task to not slow down the API response to the user.
	app.backgroundTask(r, func() error {
		return app.checkAndCompleteReferral(context.Background(), currentUser)
	})

	var uploadedFiles []uploadedFile
	for _, fheaders := range r.MultipartForm.File {
		for _, headers := range fheaders {
			var uploadedFile uploadedFile

			file, err := headers.Open()
			if err != nil {
				log.Println("error opening file at posts.go:", err)
				app.serverError(w, r, fmt.Errorf("failed to open file: %w", err))
				return
			}
			defer file.Close()

			contentBuf := bytes.NewBuffer(nil)
			if _, err := io.Copy(contentBuf, file); err != nil {
				log.Println("error reading file content:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file content: %w", err))
				return
			}
			fileContent := contentBuf.Bytes()

			contentType := http.DetectContentType(fileContent[:512])
			if contentType != "image/jpeg" && contentType != "image/png" && contentType != "video/mp4" {
				app.badRequest(w, r, fmt.Errorf("unsupported file type: %s", contentType))
				return
			}
			uploadedFile.ContentType = contentType
			log.Println("detected content type", uploadedFile.ContentType)

			uploadedFile.Size = int64(len(fileContent))
			log.Println("file size", uploadedFile.Size)

			uploadedFile.Filename = headers.Filename
			log.Println("file name", uploadedFile.Filename)

			uploadedFile.FileContent = fileContent
			uploadedFiles = append(uploadedFiles, uploadedFile)
		}
	}

	type uploadResult struct {
		url string
		err error
	}

	results := make(chan uploadResult, len(uploadedFiles))
	var wg sync.WaitGroup

	for _, file := range uploadedFiles {
		wg.Add(1)
		go func(f uploadedFile) {
			defer wg.Done()
			filePath := fmt.Sprintf("posts/%v/%v", post.Id, "media")
			url, err := app.uploadFileToCloudinary(ctx, "media", post.Id, filePath, file.FileContent)
			results <- uploadResult{url: url, err: err}
		}(file)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var urls []string
	for result := range results {
		if result.err != nil {
			log.Println("error uploading post media:", result.err)
			app.serverError(w, r, fmt.Errorf("failed to upload post media: %w", result.err))
			return
		}
		urls = append(urls, result.url)
	}

	for _, url := range urls {
		if err := app.store.Posts.AddMediaUrl(ctx, post.Id, url); err != nil {
			log.Println("error adding media url to post:", err)
			app.serverError(w, r, fmt.Errorf("failed to add media url to post: %w", err))
			return
		}
	}

	log.Println("uploaded media urls", urls)

	response.JSON(w, http.StatusCreated, post, false, "post created successfully")
}

// Logic: This function now checks the post count and creates a notification.
func (app *application) checkAndCompleteReferral(ctx context.Context, referee *store.User) error {
    postCount, err := app.store.Posts.GetPostCountByUserID(ctx, referee.Id)
    if err != nil {
        log.Printf("Could not get post count for user %d: %v", referee.Id, err)
        return err // Return error to be logged by backgroundTask
    }

    // The referral reward is only given for the *very first* post.
    if postCount != 1 {
        log.Printf("Not the first post for user %d (post count: %d). No referral action taken.", referee.Id, postCount)
        return nil
    }

	pendingReferral, err := app.store.Referrals.GetPendingByRefereeID(ctx, referee.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			log.Printf("No pending referral found for user ID %d. No action taken.", referee.Id)
			return nil
		}
		log.Printf("Error checking for pending referral for user ID %d: %v", referee.Id, err)
		return err
	}

	log.Printf("Pending referral found! ID: %d. Referrer: %d. Completing...", pendingReferral.ID, pendingReferral.ReferrerID)

	exBirdPlan, err := app.store.Users.GetSubscriptionByName(ctx, "ExBird")
	if err != nil {
		log.Printf("CRITICAL: Could not find 'ExBird' subscription plan to grant referral reward. Error: %v", err)
		return err
	}

	err = app.store.Users.GrantSubscriptionForOrder(ctx, pendingReferral.ReferrerID, exBirdPlan.ID)
	if err != nil {
		log.Printf("CRITICAL: Failed to grant subscription reward to referrer user ID %d. Error: %v", pendingReferral.ReferrerID, err)
		return err
	}
	log.Printf("Successfully granted ExBird subscription to referrer ID %d.", pendingReferral.ReferrerID)

    // Create a notification for the referrer
    notification := &store.Notification{
        UserID:  pendingReferral.ReferrerID,
        Type:    "referral_success",
        Message: fmt.Sprintf("Congratulations! You have been awarded 1 month of ExBird for referring user %s.", referee.Username),
    }
    err = app.store.Notifications.Create(ctx, notification)
    if err != nil {
        log.Printf("CRITICAL: Failed to create notification for referrer ID %d. Error: %v", pendingReferral.ReferrerID, err)
        // We continue even if notification fails, as the subscription grant is more critical.
    } else {
        log.Printf("Successfully created referral reward notification for user %d.", pendingReferral.ReferrerID)
    }

	err = app.store.Referrals.Complete(ctx, pendingReferral.ID)
	if err != nil {
		log.Printf("CRITICAL: Failed to mark referral ID %d as completed. Error: %v", pendingReferral.ID, err)
		return err
	}

	log.Printf("Referral ID %d successfully marked as completed.", pendingReferral.ID)
	return nil
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

func (app *application) getPostRetrievalStrategy(strategy string) services.PostRetriever {
	var postRetrievalStrategy services.PostRetriever
	switch strategy {
	case "trending":
		postRetrievalStrategy = services.NewTrendingPostRetriever(app.store)
	case "all":
		postRetrievalStrategy = services.NewAllPostRetriever(app.store)
	case "follower":
		postRetrievalStrategy = services.NewFollowerPostsRetriever(app.store)
	default:
		postRetrievalStrategy = services.NewAllPostRetriever(app.store)
	}

	return postRetrievalStrategy
}