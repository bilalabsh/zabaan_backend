package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

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

// maxRequestBodyBytes is the maximum size of request body for auth endpoints (1MB).
const maxRequestBodyBytes = 1 << 20

// loginRequestBody is the JSON body for Login and GetToken.
type loginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// methodNotAllowed writes 405 and returns true if r.Method != method; otherwise returns false.
func methodNotAllowed(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
		return true
	}
	return false
}

// Signup handles POST /signup.
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Password  string `json:"password"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			json.NewEncoder(w).Encode(map[string]string{"error": "request body too large"})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return
	}
	if body.FirstName == "" || body.LastName == "" || body.Email == "" || body.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "first_name, last_name, email and password required"})
		return
	}
	user, err := h.svc.SignUp(body.FirstName, body.LastName, body.Email, body.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid email format"})
			return
		}
		if errors.Is(err, ErrEmailTooLong) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "email too long"})
			return
		}
		if errors.Is(err, ErrFirstNameTooLong) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "first_name too long"})
			return
		}
		if errors.Is(err, ErrLastNameTooLong) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "last_name too long"})
			return
		}
		if errors.Is(err, ErrWeakPassword) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "password must be at least 8 characters and contain a letter and a number"})
			return
		}
		if errors.Is(err, ErrPasswordTooLong) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "password must be at most 72 characters"})
			return
		}
		if errors.Is(err, ErrEmailExists) {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "email already exists"})
			return
		}
		slog.Error("signup failed", "handler", "Signup", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return
	}
	token, err := h.svc.CreateToken(user.ID, user.Email)
	if err != nil {
		slog.Error("signup create token failed", "handler", "Signup", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create token"})
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"user": user, "token": token})
}

// bearerResult holds the result of validating an optional Bearer token.
// Rejected is true when a Bearer was sent but invalid/expired (response already written).
// Claims is set when Bearer was sent and valid; caller must ensure it matches the credential user.
type bearerResult struct {
	Claims   *Claims
	Rejected bool
}

func (h *Handler) rejectInvalidBearer(w http.ResponseWriter, r *http.Request) bearerResult {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return bearerResult{}
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	claims, err := h.svc.ValidateTokenFull(tokenString)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
		return bearerResult{Rejected: true}
	}
	return bearerResult{Claims: claims}
}

// ensureBearerMatchesUser returns true and writes 401 if Bearer was sent but belongs to a different user.
func (h *Handler) ensureBearerMatchesUser(w http.ResponseWriter, bearerClaims *Claims, userID uint) bool {
	if bearerClaims == nil {
		return false
	}
	if bearerClaims.Subject != fmt.Sprintf("%d", userID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "token does not belong to this user"})
		return true
	}
	return false
}

// authenticateWithCredentials validates method, optional Bearer, body (email/password), runs Login, and ensures Bearer matches user.
// Returns (user, true) when the handler should return (error or mismatch already written); (user, false) to continue.
func (h *Handler) authenticateWithCredentials(w http.ResponseWriter, r *http.Request, logLabel string) (*models.User, bool) {
	ber := h.rejectInvalidBearer(w, r)
	if ber.Rejected {
		return nil, true
	}
	var body loginRequestBody
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			json.NewEncoder(w).Encode(map[string]string{"error": "request body too large"})
			return nil, true
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
		return nil, true
	}
	if body.Email == "" || body.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "email and password required"})
		return nil, true
	}
	user, err := h.svc.Login(body.Email, body.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid email or password"})
			return nil, true
		}
		if errors.Is(err, ErrEmailTooLong) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "email too long"})
			return nil, true
		}
		slog.Error("login failed", "handler", logLabel, "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "internal server error"})
		return nil, true
	}
	if h.ensureBearerMatchesUser(w, ber.Claims, user.ID) {
		return nil, true
	}
	return user, false
}

// Login handles POST /login.
// Bearer is optional. If sent, it must be valid (not tampered/expired) and must refer to the same user as the credentials in the body; otherwise 401.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	user, done := h.authenticateWithCredentials(w, r, "Login")
	if done {
		return
	}
	token, err := h.svc.CreateToken(user.ID, user.Email)
	if err != nil {
		slog.Error("login create token failed", "handler", "Login", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create token"})
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"user": user, "token": token})
}

// GetToken handles POST /getToken.
// Exchanges email and password for a new token and invalidates all previously issued tokens for that user.
// Bearer is optional; if sent, must be valid and for the same user as the credentials.
func (h *Handler) GetToken(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	user, done := h.authenticateWithCredentials(w, r, "GetToken login")
	if done {
		return
	}
	issuedAt := time.Now()
	if err := h.svc.RevokePreviousTokensAt(user.ID, issuedAt); err != nil {
		slog.Error("getToken revoke previous tokens failed", "handler", "GetToken", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to revoke previous tokens"})
		return
	}
	token, err := h.svc.CreateTokenWithIssuedAt(user.ID, user.Email, issuedAt)
	if err != nil {
		slog.Error("getToken create token failed", "handler", "GetToken", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "failed to create token"})
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{"token": token})
}
