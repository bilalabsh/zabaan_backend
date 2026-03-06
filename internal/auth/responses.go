package auth

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes status and JSON body. Sets Content-Type to application/json.
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes a JSON error response with the given status and message.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// MethodNotAllowed writes 405 and returns true if r.Method != method; otherwise returns false.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return true
	}
	return false
}
