package models


type User struct {
	ID        uint   `json:"id" gorm:"primaryKey"`
	Email     string `json:"email" gorm:"unique;not null"`
	Username  string `json:"username" gorm:"unique;not null"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

