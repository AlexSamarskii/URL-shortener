CREATE TABLE IF NOT EXISTS urls (
    short_code VARCHAR(255) PRIMARY KEY,
    original_url TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_urls_original_url ON urls(original_url);