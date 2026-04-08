package domain

import "time"

// User represents a system user
type User struct {
	ID                 int        `json:"id"`
	Username           string     `json:"username"`
	PasswordHash       string     `json:"-"` // Never expose password hash in JSON
	Email              string     `json:"email,omitempty"`
	Role               string     `json:"role"` // "admin" or "user"
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastLogin          *time.Time `json:"last_login,omitempty"`
	IsActive           bool       `json:"is_active"`
	MustChangePassword bool       `json:"must_change_password"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents a successful login response
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresIn int       `json:"expires_in"` // seconds
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

// UserInfo represents public user information
type UserInfo struct {
	ID                 int        `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email,omitempty"`
	Role               string     `json:"role"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
	LastLogin          *time.Time `json:"last_login,omitempty"`
	IsActive           bool       `json:"is_active"`
	MustChangePassword bool       `json:"must_change_password"`
}

// CreateUserRequest represents a request to create a new user
type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role"` // "admin" or "user"
}

// UpdateUserRequest represents a request to update a user
type UpdateUserRequest struct {
	Email    *string `json:"email,omitempty"`
	Role     *string `json:"role,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// ChangePasswordRequest represents a password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ResetPasswordRequest represents an admin password reset request
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password"`
	ForceChange bool   `json:"force_change,omitempty"` // Force user to change password on next login
}

// UserRepository defines the persistence interface for users
type UserRepository interface {
	Create(user *User) error
	GetByID(id int) (*User, error)
	GetByUsername(username string) (*User, error)
	GetByEmail(email string) (*User, error)
	List() ([]*User, error)
	GetByRole(role string) ([]*User, error)
	Update(user *User) error
	UpdatePassword(userID int, passwordHash string) error
	UpdateLastLogin(userID int) error
	Delete(id int) error
	Count() (int, error)
	CountByRole(role string) (int, error)
}
