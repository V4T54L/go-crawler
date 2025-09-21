package postgres

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/user/crawler-service/internal/entity"
)

// ExtractedDataRepoImpl provides a concrete implementation for the ExtractedDataRepository interface using PostgreSQL.
type ExtractedDataRepoImpl struct {
	db *pgxpool.Pool
}

// NewExtractedDataRepo creates a new instance of ExtractedDataRepoImpl.
func NewExtractedDataRepo(db *pgxpool.Pool) *ExtractedDataRepoImpl {
	return &ExtractedDataRepoImpl{db: db}
}

// Save stores or updates the extracted data for a URL in the database.
func (r *ExtractedDataRepoImpl) Save(ctx context.Context, data *entity.ExtractedData) error {
	imagesJSON, err := json.Marshal(data.Images)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO extracted_data (url, title, description, keywords, h1_tags, content, images, http_status_code, response_time_ms, crawl_timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (url) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			keywords = EXCLUDED.keywords,
			h1_tags = EXCLUDED.h1_tags,
			content = EXCLUDED.content,
			images = EXCLUDED.images,
			http_status_code = EXCLUDED.http_status_code,
			response_time_ms = EXCLUDED.response_time_ms,
			crawl_timestamp = EXCLUDED.crawl_timestamp;
	`

	_, err = r.db.Exec(ctx, query,
		data.URL,
		data.Title,
		data.Description,
		data.Keywords,
		data.H1Tags,
		data.Content,
		imagesJSON,
		data.HTTPStatusCode,
		data.ResponseTimeMS,
		data.CrawlTimestamp,
	)
	return err
}

// FindByURL retrieves the extracted data for a specific URL from the database.
func (r *ExtractedDataRepoImpl) FindByURL(ctx context.Context, url string) (*entity.ExtractedData, error) {
	query := `
		SELECT id, url, title, description, keywords, h1_tags, content, images, http_status_code, response_time_ms, crawl_timestamp
		FROM extracted_data
		WHERE url = $1;
	`
	row := r.db.QueryRow(ctx, query, url)

	var data entity.ExtractedData
	var imagesJSON []byte

	err := row.Scan(
		&data.ID,
		&data.URL,
		&data.Title,
		&data.Description,
		&data.Keywords,
		&data.H1Tags,
		&data.Content,
		&imagesJSON,
		&data.HTTPStatusCode,
		&data.ResponseTimeMS,
		&data.CrawlTimestamp,
	)
	if err != nil {
		return nil, err // pgx.ErrNoRows will be returned if not found
	}

	if err := json.Unmarshal(imagesJSON, &data.Images); err != nil {
		return nil, err
	}

	return &data, nil
}

