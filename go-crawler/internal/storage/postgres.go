package storage

import (
	"context"
	"crawler/internal/domain"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore handles interactions with the PostgreSQL database.
type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(connStr string) (*PostgresStore, error) {
	db, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.db.Ping(ctx)
}

// SaveData saves extracted page data within a single transaction.
func (s *PostgresStore) SaveData(ctx context.Context, data *domain.PageData) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var pageID int
	err = tx.QueryRow(ctx,
		`INSERT INTO crawled_pages (url, title, status, fail_reason)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (url) DO UPDATE SET
		   title = EXCLUDED.title, status = EXCLUDED.status, fail_reason = EXCLUDED.fail_reason, updated_at = NOW()
		 RETURNING id`,
		data.URL, data.Title, data.Status, data.FailReason,
	).Scan(&pageID)
	if err != nil {
		return err
	}

	// Insert content
	if data.Content != "" {
		_, err = tx.Exec(ctx,
			`INSERT INTO page_content (page_id, content) VALUES ($1, $2)
			 ON CONFLICT (page_id) DO UPDATE SET content = EXCLUDED.content`,
			pageID, data.Content)
		if err != nil {
			return err
		}
	}

	// Batch insert metadata
	if len(data.MetaTags) > 0 {
		batch := &pgx.Batch{}
		for key, value := range data.MetaTags {
			batch.Queue(`INSERT INTO page_metadata (page_id, meta_key, meta_value) VALUES ($1, $2, $3)
			             ON CONFLICT (page_id, meta_key) DO UPDATE SET meta_value = EXCLUDED.meta_value`,
				pageID, key, value)
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return err
		}
	}

	// Similar batch inserts for images and headers...

	return tx.Commit(ctx)
}

// GetCrawlStatus retrieves the current status of a URL.
func (s *PostgresStore) GetCrawlStatus(ctx context.Context, url string) (*domain.CrawlStatusResponse, error) {
	var status domain.CrawlStatusResponse
	err := s.db.QueryRow(ctx,
		`SELECT url, status, fail_reason, updated_at FROM crawled_pages WHERE url = $1`,
		url,
	).Scan(&status.URL, &status.Status, &status.FailReason, &status.UpdatedAt)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("not_found")
	}
	return &status, err
}
