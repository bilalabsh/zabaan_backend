package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bilalabsh/zabaan_backend/internal/auth/dto"
	"github.com/bilalabsh/zabaan_backend/internal/models"
)

// BearerResult holds the result of validating an optional Bearer token.
// Rejected is true when a Bearer was sent but invalid/expired (response already written).
// Claims is set when Bearer was sent and valid; caller must ensure it matches the credential user.
type BearerResult struct {
	Claims   *Claims
	Rejected bool
}

// RejectInvalidBearer validates an optional Bearer token. Returns BearerResult with Rejected=true if invalid.
func RejectInvalidBearer(w http.ResponseWriter, r *http.Request, v TokenValidator) BearerResult {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return BearerResult{}
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := v.ValidateTokenFull(tokenString)
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "invalid or expired token")
		return BearerResult{Rejected: true}
	}
	return BearerResult{Claims: claims}
}

// EnsureBearerMatchesUser returns true and writes 401 if Bearer was sent but belongs to a different user.
func EnsureBearerMatchesUser(w http.ResponseWriter, bearerClaims *Claims, userID uint) bool {
	if bearerClaims == nil {
		return false
	}
	if bearerClaims.Subject != fmt.Sprintf("%d", userID) {
		WriteError(w, http.StatusUnauthorized, "token does not belong to this user")
		return true
	}
	return false
}

// MaxRequestBodyBytes is the maximum size of request body for auth endpoints (1MB).
const MaxRequestBodyBytes = 1 << 20

// AuthenticateWithCredentials validates optional Bearer, parses body (email/password), runs Login, and ensures Bearer matches user.
// Returns (user, true) when the handler should return (error or mismatch already written); (user, false) to continue.
func AuthenticateWithCredentials(w http.ResponseWriter, r *http.Request, svc AuthService, logLabel string) (*models.User, bool) {
	ber := RejectInvalidBearer(w, r, svc)
	if ber.Rejected {
		return nil, true
	}
	var body dto.LoginRequest
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return nil, true
		}
		WriteError(w, http.StatusBadRequest, "invalid JSON")
		return nil, true
	}
	if body.Email == "" || body.Password == "" {
		WriteError(w, http.StatusBadRequest, "email and password required")
		return nil, true
	}
	user, err := svc.Login(body.Email, body.Password)
	if err != nil {
		if status, msg := MapLoginError(err); status != 0 {
			WriteError(w, status, msg)
			return nil, true
		}
		slog.Error("login failed", "handler", logLabel, "err", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return nil, true
	}
	if EnsureBearerMatchesUser(w, ber.Claims, user.ID) {
		return nil, true
	}
	return user, false
}
