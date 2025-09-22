CREATE TABLE IF NOT EXISTS crawled_pages (
    id SERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    title TEXT,
    status VARCHAR(20) NOT NULL, -- 'completed', 'failed', 'processing'
    fail_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS page_content (
    page_id INTEGER PRIMARY KEY REFERENCES crawled_pages(id) ON DELETE CASCADE,
    content TEXT
);

CREATE TABLE IF NOT EXISTS page_metadata (
    id SERIAL PRIMARY KEY,
    page_id INTEGER NOT NULL REFERENCES crawled_pages(id) ON DELETE CASCADE,
    meta_key VARCHAR(255) NOT NULL,
    meta_value TEXT,
    UNIQUE(page_id, meta_key)
);

CREATE TABLE IF NOT EXISTS page_images (
    id SERIAL PRIMARY KEY,
    page_id INTEGER NOT NULL REFERENCES crawled_pages(id) ON DELETE CASCADE,
    image_url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS page_headers (
    id SERIAL PRIMARY KEY,
    page_id INTEGER NOT NULL REFERENCES crawled_pages(id) ON DELETE CASCADE,
    header_text TEXT NOT NULL
);

-- Function to automatically update the 'updated_at' timestamp
CREATE OR REPLACE FUNCTION trigger_set_timestamp()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger to execute the function before an update on crawled_pages
CREATE TRIGGER set_timestamp
BEFORE UPDATE ON crawled_pages
FOR EACH ROW
EXECUTE PROCEDURE trigger_set_timestamp();