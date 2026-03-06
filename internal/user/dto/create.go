package dto

// CreateUserRequest is the JSON body for POST /users.
type CreateUserRequest struct {
	Email    string `json:"email"`
	Username string `json:"username"`
}
