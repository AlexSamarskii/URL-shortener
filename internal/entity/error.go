package entity

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrExpired       = errors.New("expired")
	ErrAlreadyExists = errors.New("already exists")
)
