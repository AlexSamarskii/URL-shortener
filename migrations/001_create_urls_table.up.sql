CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS urls (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    short_code VARCHAR(255) NOT NULL UNIQUE,
    original_url TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_urls_short_code ON urls(short_code);
CREATE INDEX idx_urls_original_url ON urls(original_url);