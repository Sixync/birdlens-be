package mediamanager

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"
	"github.com/sixync/birdlens-be/internal/env"
)

type CloudinaryClient struct {
	cld *cloudinary.Cloudinary
	ctx context.Context
}

func NewCloudinaryClient() *CloudinaryClient {
	// Add your Cloudinary credentials, set configuration parameter
	// Secure=true to return "https" URLs, and create a context
	key := env.GetString("CLOUDINARY_API_KEY", "")
	secret := env.GetString("CLOUDINARY_API_SECRET", "")
	cloudName := env.GetString("CLOUDINARY_CLOUD_NAME", "")

	if key == "" || secret == "" || cloudName == "" {
		panic("Cloudinary credentials are not set in environment variables")
	}

	cld, err := cloudinary.NewFromParams(cloudName, key, secret)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Cloudinary client: %v", err))
	}

	cld.Config.URL.Secure = true
	ctx := context.Background()
	return &CloudinaryClient{
		cld: cld,
		ctx: ctx,
	}
}

// type MediaClient interface {
// 	Upload(ctx context.Context, filePath string, fileData []byte) (string, error)
// 	Delete(ctx context.Context, filePath string) error
// 	// GetImagesUrlByPath(ctx context.Context, path string) ([]string, error)
// }

func (client *CloudinaryClient) Upload(
	ctx context.Context,
	fileName string,
	filePath string,
	fileData any,
) (string, error) {
	uploadResult, err := client.cld.Upload.Upload(
		ctx,
		fileData,
		uploader.UploadParams{
			PublicID:       fileName,
			UniqueFilename: api.Bool(false),
			Overwrite:      api.Bool(true),
			Folder:         filePath,
		})
	if err != nil {
		log.Println("error uploading file at cloudinary.go:", err)
		return "", err
	}
	return uploadResult.SecureURL, nil
}

func (client *CloudinaryClient) Delete(
	ctx context.Context,
	filePath string,
) error {
	_, err := client.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID: filePath,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}
	return nil
}
