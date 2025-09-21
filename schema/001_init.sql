CREATE TABLE IF NOT EXISTS extracted_data (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    title TEXT,
    description TEXT,
    keywords TEXT[],
    h1_tags TEXT[],
    content TEXT,
    images JSONB,
    crawl_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    http_status_code INT,
    response_time_ms INT
);

CREATE INDEX IF NOT EXISTS idx_extracted_data_url ON extracted_data(url);
CREATE INDEX IF NOT EXISTS idx_extracted_data_crawl_timestamp ON extracted_data(crawl_timestamp);

CREATE TABLE IF NOT EXISTS failed_urls (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL UNIQUE,
    failure_reason TEXT,
    http_status_code INT,
    last_attempt_timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    retry_count INT NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_failed_urls_url ON failed_urls(url);
CREATE INDEX IF NOT EXISTS idx_failed_urls_next_retry_at ON failed_urls(next_retry_at);


