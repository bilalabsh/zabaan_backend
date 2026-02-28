package dto

// LoginRequest is the JSON body for POST /login and POST /getToken.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
