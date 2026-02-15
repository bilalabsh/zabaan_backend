package user

import (
	"database/sql"
	"errors"
	"time"

	"github.com/bilalabsh/zabaan_backend/internal/models"
	"github.com/go-sql-driver/mysql"
)

// ErrDuplicateEmail is returned when signup uses an email or username that already exists.
var ErrDuplicateEmail = errors.New("email already exists")

// Repository handles user persistence.
type Repository struct {
	db *sql.DB
}

// NewRepository returns a new user repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// List returns all users.
func (r *Repository) List() ([]models.User, error) {
	if r.db == nil {
		return nil, sql.ErrConnDone
	}
	rows, err := r.db.Query("SELECT id, email, username, first_name, last_name, created_at, updated_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Username, &u.FirstName, &u.LastName, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		u.UpdatedAt = updatedAt.Format(time.RFC3339)
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetByID returns one user by id.
func (r *Repository) GetByID(id uint) (*models.User, error) {
	if r.db == nil {
		return nil, sql.ErrNoRows
	}
	var u models.User
	var createdAt, updatedAt time.Time
	err := r.db.QueryRow("SELECT id, email, username, first_name, last_name, created_at, updated_at FROM users WHERE id = ?", int64(id)).Scan(&u.ID, &u.Email, &u.Username, &u.FirstName, &u.LastName, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = createdAt.Format(time.RFC3339)
	u.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &u, nil
}

// GetByEmail returns the user and password hash for login.
func (r *Repository) GetByEmail(email string) (*models.User, string, error) {
	if r.db == nil {
		return nil, "", sql.ErrNoRows
	}
	var u models.User
	var createdAt, updatedAt time.Time
	var passwordHash string
	err := r.db.QueryRow("SELECT id, email, username, first_name, last_name, password_hash, created_at, updated_at FROM users WHERE email = ?", email).Scan(&u.ID, &u.Email, &u.Username, &u.FirstName, &u.LastName, &passwordHash, &createdAt, &updatedAt)
	if err != nil {
		return nil, "", err
	}
	u.CreatedAt = createdAt.Format(time.RFC3339)
	u.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &u, passwordHash, nil
}

// Create inserts a user (email, username only).
func (r *Repository) Create(email, username string) (*models.User, error) {
	if r.db == nil {
		return nil, sql.ErrConnDone
	}
	res, err := r.db.Exec("INSERT INTO users (email, username) VALUES (?, ?)", email, username)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(uint(id))
}

// CreateWithPassword inserts a user with auth fields (for signup).
func (r *Repository) CreateWithPassword(email, username, firstName, lastName, passwordHash string) (*models.User, error) {
	if r.db == nil {
		return nil, sql.ErrConnDone
	}
	res, err := r.db.Exec("INSERT INTO users (email, username, first_name, last_name, password_hash) VALUES (?, ?, ?, ?, ?)", email, username, firstName, lastName, passwordHash)
	if err != nil {
		var myErr *mysql.MySQLError
		if errors.As(err, &myErr) && myErr.Number == 1062 {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return r.GetByID(uint(id))
}

// GetTokenValidAfter returns the time after which only newly issued tokens are valid (zero = no revocation).
func (r *Repository) GetTokenValidAfter(userID uint) (time.Time, error) {
	if r.db == nil {
		return time.Time{}, sql.ErrConnDone
	}
	var t sql.NullTime
	err := r.db.QueryRow("SELECT token_valid_after FROM users WHERE id = ?", int64(userID)).Scan(&t)
	if err != nil {
		return time.Time{}, err
	}
	if !t.Valid {
		return time.Time{}, nil
	}
	return t.Time, nil
}

// UpdateTokenValidAfter invalidates all tokens issued before t for this user.
// Uses a transaction with SELECT FOR UPDATE to serialize concurrent GetToken calls for the same user.
func (r *Repository) UpdateTokenValidAfter(userID uint, t time.Time) error {
	if r.db == nil {
		return sql.ErrConnDone
	}
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.Exec("SELECT id FROM users WHERE id = ? FOR UPDATE", int64(userID))
	if err != nil {
		return err
	}
	_, err = tx.Exec("UPDATE users SET token_valid_after = ? WHERE id = ?", t, int64(userID))
	if err != nil {
		return err
	}
	return tx.Commit()
}
