DROP INDEX IF EXISTS idx_urls_original_url;
DROP INDEX IF EXISTS idx_urls_short_code;
DROP TABLE IF EXISTS urls;
DROP EXTENSION IF EXISTS "uuid-ossp";