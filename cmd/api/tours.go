package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/sixync/birdlens-be/internal/request"
	"github.com/sixync/birdlens-be/internal/response"
	"github.com/sixync/birdlens-be/internal/store"
	"github.com/sixync/birdlens-be/internal/validator"
)

var (
	TourKey              key = "tour"
	MaxMultipartFileSize     = int64(100 << 20) // 30 MB
)

type TourResponse struct {
	TourName        string   `json:"tour_name"`
	Rating          float64  `json:"rating"`
	NumberOfRatings int64    `json:"number_of_ratings"`
	StoreAvatarUrl  *string  `json:"store_avatar_url"`
	StoreName       string   `json:"store_name"`
	TourDescription string   `json:"tour_description"`
	TourImagesUrl   []string `json:"tour_images_url"`
}

func (app *application) getToursHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	limit, offset := getPaginateFromCtx(r)
	ctx := r.Context()
	tours, err := app.store.Tours.GetAll(ctx, limit, offset)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, tours, false, "get tours successfully")
}

func (app *application) getTourHandler(w http.ResponseWriter, r *http.Request) {
	tour, err := app.getTourFromContext(r)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()
	event, err := app.store.Events.GetByID(ctx, tour.EventId)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	tour.Event = event

	location, err := app.store.Location.GetByID(ctx, tour.LocationId)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	tour.Location = location

	// get tour images
	urls, err := app.store.Tours.GetTourImagesUrl(ctx, tour.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	tour.ImagesUrl = urls

	response.JSON(w, http.StatusOK, tour, false, "get tour successfully")
}

type TourCreateRequest struct {
	EventID         int64   `json:"event_id"`
	TourName        string  `json:"tour_name" validate:"required"`
	TourThumbnail   []byte  `json:"tour_thumbnail"`
	TourDescription string  `json:"tour_description"`
	Price           float64 `json:"price" validate:"required"`
	TourCapacity    int     `json:"tour_capacity"`
	DurationDays    int     `json:"duration_days" validate:"required"`
	LocationId      int64   `json:"location_id" validate:"required"`
}

func (app *application) createTourHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req TourCreateRequest
	if err := request.DecodeJSON(w, r, &req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	if err := validator.Validate(req); err != nil {
		app.badRequest(w, r, err)
		return
	}

	ctx := r.Context()

	tour := &store.Tour{
		EventId:     req.EventID,
		Name:        req.TourName,
		Description: req.TourDescription,
		Price:       req.Price,
		Capacity:    req.TourCapacity,
		Duration:    req.DurationDays,
		LocationId:  req.LocationId,
	}

	log.Println(tour)

	// TODO: thumbnail and images
	// if len(req.TourThumbnail) > 0 {
	// 	imagePath := fmt.Sprintf("tours/%v/thumbnail", tour.ID)
	// 	thumbnailUrl, err := app.mediaClient.Upload(ctx, imagePath, req.TourThumbnail)
	// 	log.Println("thumbnail url", thumbnailUrl)
	// 	if err != nil {
	// 		app.serverError(w, r, err)
	// 		return
	// 	}
	// 	tour.ThumbnailUrl = &thumbnailUrl
	// }

	err := app.store.Tours.Create(ctx, tour)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, tour, false, "tour created successfully")
}

type uploadedFile struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename"`
	FileContent []byte `json:"file_content"`
}

func (app *application) addTourImagesHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	tour, err := app.getTourFromContext(r)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}

	log.Println("tour from context", tour)

	err = r.ParseMultipartForm(MaxMultipartFileSize)
	if err != nil {
		app.badRequest(w, r, fmt.Errorf("failed to parse multipart form: %w", err))
		return
	}

	ctx := r.Context()

	var uploadedImages []uploadedFile

	for _, fheaders := range r.MultipartForm.File {
		for _, headers := range fheaders {
			// process each files
			var uploadedImage uploadedFile

			file, err := headers.Open()
			if err != nil {
				log.Println("error opening file at tours.go:", err)
				app.serverError(w, r, fmt.Errorf("failed to open file: %w", err))
				return
			}

			defer file.Close()

			// detect contentType

			buff := make([]byte, 512)

			file.Read(buff)

			file.Seek(0, 0)

			contentType := http.DetectContentType(buff)
			if contentType != "image/jpeg" && contentType != "image/png" {
				app.badRequest(w, r, fmt.Errorf("unsupported file type: %s", contentType))
				return
			}

			uploadedImage.ContentType = contentType
			log.Println("detected content type", uploadedImage.ContentType)

			// get file size

			var sizeBuff bytes.Buffer
			fileSize, err := sizeBuff.ReadFrom(file)
			if err != nil {
				log.Println("error reading file size:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file size: %w", err))
				return
			}

			file.Seek(0, 0)

			uploadedImage.Size = fileSize
			log.Println("file size", uploadedImage.Size)

			uploadedImage.Filename = headers.Filename
			log.Println("file name", uploadedImage.Filename)

			contentBuf := bytes.NewBuffer(nil)

			if _, err := io.Copy(contentBuf, file); err != nil {
				log.Println("error reading file content:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file content: %w", err))
				return
			}

			uploadedImage.FileContent = contentBuf.Bytes()
			uploadedImages = append(uploadedImages, uploadedImage)
		}
	}

	var urls []string
	for _, uploadedImage := range uploadedImages {
		filePath := fmt.Sprintf("tours/%v/images/%v", tour.ID, uploadedImage.Filename)
		url, err := app.uploadFileToCloudinary(ctx, "images", tour.ID, filePath, uploadedImage.FileContent)
		if err != nil {
			log.Println("error uploading tour image:", err)
			app.serverError(w, r, fmt.Errorf("failed to upload tour image: %w", err))
			return
		}

		err = app.store.Tours.AddTourImagesUrl(ctx, tour.ID, url)
		if err != nil {
			log.Println("error adding tour images url in the addtourimage loop:", err)
			app.serverError(w, r, fmt.Errorf("failed to add tour images url: %w", err))
			return
		}
		urls = append(urls, url)
	}

	log.Println("uploaded images urls", urls)

	response.JSON(w, http.StatusOK, urls, false, "tour images added successfully")
}

func (app *application) addTourThumbnailHandler(
	w http.ResponseWriter,
	r *http.Request,
) {
	tour, err := app.getTourFromContext(r)
	if err != nil {
		app.badRequest(w, r, err)
		return
	}
	log.Println("tour from context", tour)
	err = r.ParseMultipartForm(MaxMultipartFileSize)
	if err != nil {
		app.badRequest(w, r, fmt.Errorf("failed to parse multipart form: %w", err))
		return
	}
	ctx := r.Context()
	var uploadedImage uploadedFile
	for _, fheaders := range r.MultipartForm.File {
		for _, headers := range fheaders {
			// process each file
			file, err := headers.Open()
			if err != nil {
				log.Println("error opening file at tours.go:", err)
				app.serverError(w, r, fmt.Errorf("failed to open file: %w", err))
				return
			}
			defer file.Close()

			// detect contentType
			buff := make([]byte, 512)
			file.Read(buff)
			file.Seek(0, 0)

			contentType := http.DetectContentType(buff)
			if contentType != "image/jpeg" && contentType != "image/png" {
				app.badRequest(w, r, fmt.Errorf("unsupported file type: %s", contentType))
				return
			}
			uploadedImage.ContentType = contentType

			// get file size
			var sizeBuff bytes.Buffer
			fileSize, err := sizeBuff.ReadFrom(file)
			if err != nil {
				log.Println("error reading file size:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file size: %w", err))
				return
			}
			file.Seek(0, 0)

			uploadedImage.Size = fileSize
			uploadedImage.Filename = headers.Filename

			contentBuf := bytes.NewBuffer(nil)
			if _, err := io.Copy(contentBuf, file); err != nil {
				log.Println("error reading file content:", err)
				app.serverError(w, r, fmt.Errorf("failed to read file content: %w", err))
				return
			}
			uploadedImage.FileContent = contentBuf.Bytes()
		}
	}
	if len(uploadedImage.FileContent) == 0 {
		app.badRequest(w, r, errors.New("no file content found"))
		return
	}

	filePath := fmt.Sprintf("tours/%v/thumbnail", tour.ID)
	url, err := app.uploadFileToCloudinary(ctx, "images", tour.ID, filePath, uploadedImage.FileContent)
	if err != nil {
		log.Println("error uploading thumbnail image:", err)
		app.serverError(w, r, fmt.Errorf("failed to upload thumbnail image: %w", err))
		return
	}

	tour.ThumbnailUrl = &url
	err = app.store.Tours.Update(ctx, tour)
	if err != nil {
		log.Println("error updating thumbnail url:", err)
		app.serverError(w, r, fmt.Errorf("failed to add tour thumbnail url: %w", err))
		return
	}
	log.Println("uploaded thumbnail url", url)
	response.JSON(w, http.StatusOK, url, false, "tour thumbnail added successfully")
}

// get tour middleware
func (app *application) getTourMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tourIdStr := r.PathValue("tour_id")
		if tourIdStr == "" {
			app.badRequest(w, r, errors.New("tour_id is required"))
			return
		}

		tourId, err := strconv.ParseInt(tourIdStr, 10, 64)
		if err != nil {
			app.badRequest(w, r, err)
			return
		}

		tour, err := app.store.Tours.GetByID(r.Context(), tourId)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		if tour == nil {
			app.notFound(w, r)
			return
		}

		ctx := context.WithValue(r.Context(), TourKey, tour)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *application) getTourFromContext(r *http.Request) (*store.Tour, error) {
	tour, ok := r.Context().Value(TourKey).(*store.Tour)
	if !ok {
		return nil, errors.New("tour not found in context")
	}

	return tour, nil
}

// TODO: Use fileType to determine the type of file being uploaded
func (app *application) uploadFileToCloudinary(
	ctx context.Context,
	fileType string,
	objectId int64,
	filePath string,
	fileData []byte,
) (string, error) {
	// object id to string
	objectIdStr := strconv.FormatInt(objectId, 10)

	// TODO: refactor this function to use upload different media types (videos,...)
	imageBase64 := fmt.Sprintf("data:image/jpeg;base64,%v", base64.StdEncoding.EncodeToString(fileData))
	imageUrl, err := app.mediaClient.Upload(ctx, objectIdStr, filePath, imageBase64)
	if err != nil {
		return "", fmt.Errorf("failed to upload image: %w", err)
	}
	return imageUrl, nil
}
