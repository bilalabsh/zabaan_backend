package auth

import (
	"database/sql"
	"errors"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bilalabsh/zabaan_backend/internal/models"
	"github.com/bilalabsh/zabaan_backend/internal/user"
	"golang.org/x/crypto/bcrypt"
)

// ErrInvalidCredentials is returned when login fails.
var ErrInvalidCredentials = errors.New("invalid email or password")

// ErrEmailExists is returned when signup uses an email that already exists.
var ErrEmailExists = errors.New("email already exists")

// ErrInvalidEmail is returned when the email format is invalid.
var ErrInvalidEmail = errors.New("invalid email format")

// ErrWeakPassword is returned when the password does not meet policy (min length, letter, number).
var ErrWeakPassword = errors.New("password does not meet requirements")

// ErrPasswordTooLong is returned when the password exceeds bcrypt's 72-byte limit.
var ErrPasswordTooLong = errors.New("password too long")

// ErrEmailTooLong is returned when the email exceeds the maximum allowed length.
var ErrEmailTooLong = errors.New("email too long")

// ErrFirstNameTooLong is returned when first_name exceeds the maximum allowed length.
var ErrFirstNameTooLong = errors.New("first_name too long")

// ErrLastNameTooLong is returned when last_name exceeds the maximum allowed length.
var ErrLastNameTooLong = errors.New("last_name too long")

// ErrTokenRevoked is returned when the token was valid but has been revoked (e.g. after GetToken).
var ErrTokenRevoked = errors.New("token revoked")

const minPasswordLength = 8
const maxPasswordBytes = 72 // bcrypt truncates at 72 bytes
const bcryptCost = 12
const maxEmailLength = 255
const maxFirstNameLength = 100
const maxLastNameLength = 100

// TokenValidator validates a Bearer token (including revocation). Used by middleware so it can depend on an interface.
type TokenValidator interface {
	ValidateTokenFull(tokenString string) (*Claims, error)
}

// UserRepository is the subset of user persistence needed by the auth service. Accepting an interface allows tests to use a mock.
type UserRepository interface {
	CreateWithPassword(email, username, firstName, lastName, passwordHash string) (*models.User, error)
	GetByEmail(email string) (*models.User, string, error)
	GetTokenValidAfter(userID uint) (time.Time, error)
	UpdateTokenValidAfter(userID uint, t time.Time) error
}

// Service holds auth use-case logic (signup, login).
type Service struct {
	userRepo            UserRepository
	jwtSecret           string
	tokenExpiry         time.Duration
	revocationTolerance time.Duration
}

// NewService returns a new auth service. tokenExpiry is the JWT lifetime (e.g. 24h); revocationTolerance is the time tolerance when comparing token iat to token_valid_after.
func NewService(userRepo UserRepository, jwtSecret string, tokenExpiry, revocationTolerance time.Duration) *Service {
	return &Service{
		userRepo:            userRepo,
		jwtSecret:           jwtSecret,
		tokenExpiry:         tokenExpiry,
		revocationTolerance: revocationTolerance,
	}
}

// NormalizeEmail returns email trimmed and lowercased for storage and lookup.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmail returns ErrInvalidEmail if the email format is invalid.
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return ErrInvalidEmail
	}
	_, err := mail.ParseAddress(email)
	if err != nil {
		return ErrInvalidEmail
	}
	return nil
}

// ValidatePassword returns ErrWeakPassword if the password does not meet policy, or ErrPasswordTooLong if over 72 bytes (bcrypt limit).
func ValidatePassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrWeakPassword
	}
	if len(password) > maxPasswordBytes {
		return ErrPasswordTooLong
	}
	var hasLetter, hasNumber bool
	for _, r := range password {
		if unicode.IsLetter(r) {
			hasLetter = true
		}
		if unicode.IsNumber(r) {
			hasNumber = true
		}
		if hasLetter && hasNumber {
			break
		}
	}
	if !hasLetter || !hasNumber {
		return ErrWeakPassword
	}
	return nil
}

// SignUp registers a user and returns the created user.
// Email is normalized (trimmed, lowercased) for storage and uniqueness.
func (s *Service) SignUp(firstName, lastName, email, password string) (*models.User, error) {
	email = NormalizeEmail(email)
	if err := ValidateEmail(email); err != nil {
		return nil, err
	}
	if len(email) > maxEmailLength {
		return nil, ErrEmailTooLong
	}
	if len(firstName) > maxFirstNameLength {
		return nil, ErrFirstNameTooLong
	}
	if len(lastName) > maxLastNameLength {
		return nil, ErrLastNameTooLong
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, err
	}
	username := email
	u, err := s.userRepo.CreateWithPassword(email, username, firstName, lastName, string(hash))
	if err != nil {
		if errors.Is(err, user.ErrDuplicateEmail) {
			return nil, ErrEmailExists
		}
		return nil, err
	}
	return u, nil
}

// Login validates credentials and returns the user.
// Email is normalized (trimmed, lowercased) for lookup.
func (s *Service) Login(email, password string) (*models.User, error) {
	email = NormalizeEmail(email)
	if len(email) > maxEmailLength {
		return nil, ErrEmailTooLong
	}
	u, hash, err := s.userRepo.GetByEmail(email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if hash == "" {
		return nil, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// CreateToken issues a JWT for the user.
func (s *Service) CreateToken(userID uint, email string) (string, error) {
	return CreateToken(s.jwtSecret, userID, email, s.tokenExpiry)
}

// CreateTokenWithIssuedAt issues a JWT with the given IssuedAt (use with RevokePreviousTokensAt so the new token is not revoked).
func (s *Service) CreateTokenWithIssuedAt(userID uint, email string, issuedAt time.Time) (string, error) {
	return CreateTokenWithIssuedAt(s.jwtSecret, userID, email, issuedAt, s.tokenExpiry)
}

// RevokePreviousTokensAt invalidates all tokens issued before t. Use the same t when creating the new token so the new token is valid.
func (s *Service) RevokePreviousTokensAt(userID uint, t time.Time) error {
	return s.userRepo.UpdateTokenValidAfter(userID, t)
}

// ValidateTokenFull validates the JWT and checks revocation (only tokens issued after token_valid_after are valid).
func (s *Service) ValidateTokenFull(tokenString string) (*Claims, error) {
	claims, err := ValidateToken(s.jwtSecret, tokenString)
	if err != nil {
		return nil, err
	}
	userID64, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	validAfter, err := s.userRepo.GetTokenValidAfter(uint(userID64))
	if err != nil {
		return nil, err
	}
	if !validAfter.IsZero() && claims.IssuedAt != nil {
		tokenSec := claims.IssuedAt.Time.Truncate(time.Second)
		validSec := validAfter.Truncate(time.Second)
		if tokenSec.Add(s.revocationTolerance).Before(validSec) {
			return nil, ErrTokenRevoked
		}
	}
	return claims, nil
}

// ValidateBearer returns nil if the token is valid (including revocation check).
func (s *Service) ValidateBearer(tokenString string) error {
	_, err := s.ValidateTokenFull(tokenString)
	return err
}

