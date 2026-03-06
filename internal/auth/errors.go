package auth

import "errors"

// MapSignupError returns (status, message) for known signup errors. Returns (0, "") for unknown errors.
func MapSignupError(err error) (status int, msg string) {
	switch {
	case errors.Is(err, ErrInvalidEmail):
		return 400, "invalid email format"
	case errors.Is(err, ErrEmailTooLong):
		return 400, "email too long"
	case errors.Is(err, ErrFirstNameTooLong):
		return 400, "first_name too long"
	case errors.Is(err, ErrLastNameTooLong):
		return 400, "last_name too long"
	case errors.Is(err, ErrWeakPassword):
		return 400, "password must be at least 8 characters and contain a letter and a number"
	case errors.Is(err, ErrPasswordTooLong):
		return 400, "password must be at most 72 characters"
	case errors.Is(err, ErrEmailExists):
		return 409, "email already exists"
	default:
		return 0, ""
	}
}

// MapLoginError returns (status, message) for known login/credential errors. Returns (0, "") for unknown errors.
func MapLoginError(err error) (status int, msg string) {
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		return 401, "invalid email or password"
	case errors.Is(err, ErrEmailTooLong):
		return 400, "email too long"
	default:
		return 0, ""
	}
}
