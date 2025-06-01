package mediamanager

import (
	"context"
)

type MediaClient interface {
	Upload(ctx context.Context, fileName string, filePath string, fileData any) (string, error)
	Delete(ctx context.Context, filePath string) error
}
