package dto

import "time"

type ShortenRequest struct {
	URL       string  `json:"url" binding:"required"`
	ExpiresIn *int    `json:"expires_in,omitempty"`
	Alias     *string `json:"alias,omitempty"`
}

type ShortenResponse struct {
	ShortCode string     `json:"short_code"`
	ShortURL  string     `json:"short_url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}
