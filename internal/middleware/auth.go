package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bilalabsh/zabaan_backend/internal/auth"
)

type contextKey string

const claimsContextKey contextKey = "claims"

// RequireAuth wraps a handler and returns 401 if the request has no valid Bearer token (including revocation check).
// On success, the JWT claims are stored in the request context; use GetClaimsFromRequest to read them.
func RequireAuth(v auth.TokenValidator, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if v == nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "auth not configured"})
			return
		}
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "missing or invalid Authorization header"})
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := v.ValidateTokenFull(tokenString)
		if err != nil {
			if errors.Is(err, auth.ErrTokenRevoked) {
				slog.Info("auth rejected", "component", "RequireAuth", "reason", "token revoked")
			} else if errors.Is(err, auth.ErrTokenInvalid) {
				slog.Info("auth rejected", "component", "RequireAuth", "reason", "invalid token", "err", err)
			} else {
				slog.Error("auth validation failed", "component", "RequireAuth", "err", err)
			}
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired token"})
			return
		}
		ctx := context.WithValue(r.Context(), claimsContextKey, claims)
		next(w, r.WithContext(ctx))
	}
}

// GetClaimsFromRequest returns the JWT claims from the request context, or nil if not authenticated.
func GetClaimsFromRequest(r *http.Request) *auth.Claims {
	c, _ := r.Context().Value(claimsContextKey).(*auth.Claims)
	return c
}
