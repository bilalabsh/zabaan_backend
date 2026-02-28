package auth

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bilalabsh/zabaan_backend/internal/auth/dto"
	"github.com/bilalabsh/zabaan_backend/internal/models"
)

// AuthService is the subset of auth operations needed by the HTTP handler. Accepting an interface allows tests to use a mock.
type AuthService interface {
	SignUp(firstName, lastName, email, password string) (*models.User, error)
	Login(email, password string) (*models.User, error)
	CreateToken(userID uint, email string) (string, error)
	CreateTokenWithIssuedAt(userID uint, email string, issuedAt time.Time) (string, error)
	RevokePreviousTokensAt(userID uint, t time.Time) error
	ValidateTokenFull(tokenString string) (*Claims, error)
}

// Handler handles auth HTTP endpoints (signup, login, getToken).
// All responses are JSON; handlers set Content-Type: application/json at the start.
type Handler struct {
	svc AuthService
}

// NewHandler returns a new auth handler.
func NewHandler(svc AuthService) *Handler {
	return &Handler{svc: svc}
}

// Signup handles POST /signup.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if MethodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	var body dto.SignupRequest
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			WriteError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.FirstName == "" || body.LastName == "" || body.Email == "" || body.Password == "" {
		WriteError(w, http.StatusBadRequest, "first_name, last_name, email and password required")
		return
	}

	user, err := h.svc.SignUp(body.FirstName, body.LastName, body.Email, body.Password)
	if err != nil {
		if status, msg := MapSignupError(err); status != 0 {
			WriteError(w, status, msg)
			return
		}
		slog.Error("signup failed", "handler", "Signup", "err", err)
		WriteError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	token, err := h.svc.CreateToken(user.ID, user.Email)
	if err != nil {
		slog.Error("signup create token failed", "handler", "Signup", "err", err)
		WriteError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	WriteJSON(w, http.StatusCreated, map[string]interface{}{"user": user, "token": token})
}

// Login handles POST /login.
// Bearer is optional. If sent, it must be valid (not tampered/expired) and must refer to the same user as the credentials in the body; otherwise 401.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if MethodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	user, done := AuthenticateWithCredentials(w, r, h.svc, "Login")
	if done {
		return
	}

	token, err := h.svc.CreateToken(user.ID, user.Email)
	if err != nil {
		slog.Error("login create token failed", "handler", "Login", "err", err)
		WriteError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"user": user, "token": token})
}

// GetToken handles POST /getToken.
// Exchanges email and password for a new token and invalidates all previously issued tokens for that user.
// Bearer is optional; if sent, must be valid and for the same user as the credentials.
func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	if MethodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")

	user, done := AuthenticateWithCredentials(w, r, h.svc, "GetToken login")
	if done {
		return
	}

	issuedAt := time.Now()
	if err := h.svc.RevokePreviousTokensAt(user.ID, issuedAt); err != nil {
		slog.Error("getToken revoke previous tokens failed", "handler", "GetToken", "err", err)
		WriteError(w, http.StatusInternalServerError, "failed to revoke previous tokens")
		return
	}

	token, err := h.svc.CreateTokenWithIssuedAt(user.ID, user.Email, issuedAt)
	if err != nil {
		slog.Error("getToken create token failed", "handler", "GetToken", "err", err)
		WriteError(w, http.StatusInternalServerError, "failed to create token")
		return
	}

	w.Header().Set("Authorization", "Bearer "+token)
	WriteJSON(w, http.StatusOK, map[string]interface{}{"token": token})
}
