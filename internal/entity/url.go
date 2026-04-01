package entity

import (
	"time"
)

type URL struct {
	ID          string
	ShortCode   string
	OriginalURL string
	ExpiresAt   *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
