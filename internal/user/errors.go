package user

import (
	"database/sql"
	"errors"
)

// MapCreateError returns (status, message) for known create-user errors. Returns (0, "") for unknown errors.
func MapCreateError(err error) (status int, msg string) {
	switch {
	case errors.Is(err, ErrDuplicateEmail):
		return 409, "email or username already exists"
	default:
		return 0, ""
	}
}

// MapGetByIDError returns (status, message) for GetByID errors. Returns (0, "") for unknown errors.
func MapGetByIDError(err error) (status int, msg string) {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return 404, "user not found"
	default:
		return 0, ""
	}
}
