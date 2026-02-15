package user

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
)

// Handler handles user HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler returns a new user handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Users handles /users and /users/:id (GET list, GET one, POST create).
func (h *Handler) Users(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := strings.TrimPrefix(r.URL.Path, "/users")
	path = strings.TrimPrefix(path, "/")
	if path != "" {
		id, err := strconv.ParseUint(path, 10, 64)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		user, err := h.svc.GetByID(uint(id))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(map[string]string{"error": "user not found"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(user)
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, err := h.svc.List()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(users)
	case http.MethodPost:
		var body struct {
			Email    string `json:"email"`
			Username string `json:"username"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid JSON"})
			return
		}
		if body.Email == "" || body.Username == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "email and username required"})
			return
		}
		user, err := h.svc.Create(body.Email, body.Username)
		if err != nil {
			if strings.Contains(err.Error(), "Duplicate") {
				w.WriteHeader(http.StatusConflict)
				json.NewEncoder(w).Encode(map[string]string{"error": "email or username already exists"})
				return
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(map[string]string{"error": "method not allowed"})
	}
}
