package repository

import (
	"context"
	"github.com/user/crawler-service/internal/entity"
)

// ExtractedDataRepository defines the interface for storing and retrieving extracted page data.
type ExtractedDataRepository interface {
	// Save stores the extracted data for a URL. If the URL already exists, it should be updated.
	Save(ctx context.Context, data *entity.ExtractedData) error
	// FindByURL retrieves the extracted data for a specific URL.
	FindByURL(ctx context.Context, url string) (*entity.ExtractedData, error)
}

