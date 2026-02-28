package user

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/bilalabsh/zabaan_backend/internal/user/dto"
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
		h.handleUserByID(w, path)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleList(w)
	case http.MethodPost:
		h.handleCreate(w, r)
	default:
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) handleUserByID(w http.ResponseWriter, path string) {
	id, err := strconv.ParseUint(path, 10, 64)
	if err != nil {
		WriteError(w, http.StatusNotFound, "not found")
		return
	}

	user, err := h.svc.GetByID(uint(id))
	if err != nil {
		if status, msg := MapGetByIDError(err); status != 0 {
			WriteError(w, status, msg)
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusOK, user)
}

func (h *Handler) handleList(w http.ResponseWriter) {
	users, err := h.svc.List()
	if err != nil {
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	WriteJSON(w, http.StatusOK, users)
}

func (h *Handler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var body dto.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.Email == "" || body.Username == "" {
		WriteError(w, http.StatusBadRequest, "email and username required")
		return
	}

	user, err := h.svc.Create(body.Email, body.Username)
	if err != nil {
		if status, msg := MapCreateError(err); status != 0 {
			WriteError(w, status, msg)
			return
		}
		WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	WriteJSON(w, http.StatusCreated, user)
}
