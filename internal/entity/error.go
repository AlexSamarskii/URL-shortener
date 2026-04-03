package entity

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrExpired       = errors.New("expired")
	ErrAlreadyExists = errors.New("already exists")
)

var (
	ErrURLInvalid        = errors.New("invalid URL")
	ErrAliasExists       = errors.New("alias already exists")
	ErrURLNotFound       = errors.New("url not found")
	ErrURLExpired        = errors.New("url expired")
	ErrGenerateCode      = errors.New("failed to generate unique code")
	ErrOriginalURLExists = errors.New("original URL already exists")
)
