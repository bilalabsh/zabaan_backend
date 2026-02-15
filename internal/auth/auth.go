package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrTokenInvalid is returned when the token is malformed, expired, or otherwise invalid (not revocation).
var ErrTokenInvalid = errors.New("invalid token")

// Claims holds JWT claims (sub = user id, email, exp).
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
}

// CreateToken signs a new JWT for the user. Expiry is the token lifetime from now.
func CreateToken(secret string, userID uint, email string, expiry time.Duration) (string, error) {
	return CreateTokenWithIssuedAt(secret, userID, email, time.Now(), expiry)
}

// CreateTokenWithIssuedAt signs a JWT with a specific IssuedAt (so token_valid_after and iat stay in sync). Expiry is the token lifetime from issuedAt.
func CreateTokenWithIssuedAt(secret string, userID uint, email string, issuedAt time.Time, expiry time.Duration) (string, error) {
	if secret == "" {
		return "", errors.New("JWT secret is empty")
	}
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   fmt.Sprintf("%d", userID),
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates the token, returns claims or error.
func ValidateToken(secret string, tokenString string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("JWT secret is empty")
	}
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("invalid signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// UserIDFromClaims returns the user ID from claims.Subject, or 0 if invalid.
func UserIDFromClaims(claims *Claims) uint {
	if claims == nil {
		return 0
	}
	var id uint
	_, _ = fmt.Sscanf(claims.Subject, "%d", &id)
	return id
}
