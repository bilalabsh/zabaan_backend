package user

import (
	"github.com/bilalabsh/zabaan_backend/internal/models"
)

// Service holds user use-case logic.
type Service struct {
	repo *Repository
}

// NewService returns a new user service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns all users.
func (s *Service) List() ([]models.User, error) {
	users, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	if users == nil {
		users = []models.User{}
	}
	return users, nil
}

// GetByID returns one user by id.
func (s *Service) GetByID(id uint) (*models.User, error) {
	return s.repo.GetByID(id)
}

// Create creates a user (email, username).
func (s *Service) Create(email, username string) (*models.User, error) {
	return s.repo.Create(email, username)
}
