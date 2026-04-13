package domain

import "errors"

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrUserInactive    = errors.New("user inactive")
	ErrUserExists      = errors.New("user already exists")
	ErrInvalidPassword = errors.New("invalid password")
	ErrTokenExpired    = errors.New("token expired")
	ErrTokenInvalid    = errors.New("token invalid")
	ErrTokenRevoked    = errors.New("token revoked")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidInput    = errors.New("invalid input")
)
