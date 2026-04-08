package services

import (
	"fmt"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the cost factor for bcrypt password hashing
	BcryptCost = 12

	// UserTokenExpiry is the expiration time for user JWT tokens (24 hours)
	UserTokenExpiry = 24 * time.Hour

	// DefaultAdminUsername is the default admin username
	DefaultAdminUsername = "admin"

	// DefaultAdminPassword is the default admin password
	DefaultAdminPassword = "changeme"
)

// UserClaims represents JWT claims for user tokens
type UserClaims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Type     string `json:"type"` // "user" to distinguish from agent tokens
	jwt.RegisteredClaims
}

// UserService handles user authentication and management
type UserService struct {
	userStore domain.UserRepository
	jwtSecret string
}

// NewUserService creates a new user service
func NewUserService(userStore domain.UserRepository, jwtSecret string) *UserService {
	return &UserService{
		userStore: userStore,
		jwtSecret: jwtSecret,
	}
}

// HashPassword hashes a password using bcrypt
func (s *UserService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash
func (s *UserService) VerifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Authenticate authenticates a user with username and password
func (s *UserService) Authenticate(username, password string) (*domain.User, error) {
	// Get user by username
	user, err := s.userStore.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if user is active
	if !user.IsActive {
		return nil, fmt.Errorf("user account is disabled")
	}

	// Verify password
	if err := s.VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Update last login
	if err := s.userStore.UpdateLastLogin(user.ID); err != nil {
		// Log error but don't fail authentication
		fmt.Printf("Warning: failed to update last login for user %s: %v\n", username, err)
	}

	return user, nil
}

// GenerateToken generates a JWT token for a user
func (s *UserService) GenerateToken(user *domain.User) (string, error) {
	claims := UserClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Type:     "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(UserTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "lixy-controller",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims
func (s *UserService) ValidateToken(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify token type
	if claims.Type != "user" {
		return nil, fmt.Errorf("invalid token type")
	}

	return claims, nil
}

// CreateUser creates a new user
func (s *UserService) CreateUser(req *domain.CreateUserRequest) (*domain.User, error) {
	// Hash password
	passwordHash, err := s.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	// Create user
	user := &domain.User{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Role:         req.Role,
		IsActive:     true,
	}

	if err := s.userStore.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(id int) (*domain.User, error) {
	return s.userStore.GetByID(id)
}

// GetUserByUsername retrieves a user by username
func (s *UserService) GetUserByUsername(username string) (*domain.User, error) {
	return s.userStore.GetByUsername(username)
}

// ListUsers retrieves all users
func (s *UserService) ListUsers() ([]*domain.User, error) {
	return s.userStore.List()
}

// UpdateUser updates a user
func (s *UserService) UpdateUser(id int, req *domain.UpdateUserRequest) (*domain.User, error) {
	// Get existing user
	user, err := s.userStore.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Update fields
	if req.Email != nil {
		user.Email = *req.Email
	}
	if req.Role != nil {
		user.Role = *req.Role
	}
	if req.IsActive != nil {
		user.IsActive = *req.IsActive
	}

	// Save changes
	if err := s.userStore.Update(user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

// ChangePassword changes a user's password
func (s *UserService) ChangePassword(userID int, req *domain.ChangePasswordRequest) error {
	// Get user
	user, err := s.userStore.GetByID(userID)
	if err != nil {
		return err
	}

	// Verify current password
	if err := s.VerifyPassword(req.CurrentPassword, user.PasswordHash); err != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Hash new password
	newPasswordHash, err := s.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	// Update password
	if err := s.userStore.UpdatePassword(userID, newPasswordHash); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ResetPassword resets a user's password (admin only)
func (s *UserService) ResetPassword(userID int, req *domain.ResetPasswordRequest) error {
	// Hash new password
	newPasswordHash, err := s.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	// Update password
	if err := s.userStore.UpdatePassword(userID, newPasswordHash); err != nil {
		return fmt.Errorf("failed to reset password: %w", err)
	}

	// If force change is requested, update the flag
	if req.ForceChange {
		user, err := s.userStore.GetByID(userID)
		if err != nil {
			return err
		}
		user.MustChangePassword = true
		if err := s.userStore.Update(user); err != nil {
			return fmt.Errorf("failed to set force change flag: %w", err)
		}
	}

	return nil
}

// DeleteUser deletes a user
func (s *UserService) DeleteUser(id int) error {
	return s.userStore.Delete(id)
}

// InitializeDefaultAdmin creates the default admin user if no admin users exist
func (s *UserService) InitializeDefaultAdmin() error {
	// Check if any admin users exist
	adminCount, err := s.userStore.CountByRole("admin")
	if err != nil {
		return fmt.Errorf("failed to check admin users: %w", err)
	}

	// If admin users exist, nothing to do
	if adminCount > 0 {
		return nil
	}

	// Create default admin user
	passwordHash, err := s.HashPassword(DefaultAdminPassword)
	if err != nil {
		return fmt.Errorf("failed to hash default admin password: %w", err)
	}

	admin := &domain.User{
		Username:           DefaultAdminUsername,
		PasswordHash:       passwordHash,
		Role:               "admin",
		IsActive:           true,
		MustChangePassword: true, // Force password change on first login
	}

	if err := s.userStore.Create(admin); err != nil {
		return fmt.Errorf("failed to create default admin user: %w", err)
	}

	fmt.Printf("Default admin user created: username=%s, password=%s (must be changed on first login)\n",
		DefaultAdminUsername, DefaultAdminPassword)

	return nil
}

// ToUserInfo converts a User to UserInfo (without sensitive data)
func (s *UserService) ToUserInfo(user *domain.User) *domain.UserInfo {
	return &domain.UserInfo{
		ID:                 user.ID,
		Username:           user.Username,
		Email:              user.Email,
		Role:               user.Role,
		CreatedAt:          user.CreatedAt,
		UpdatedAt:          user.UpdatedAt,
		LastLogin:          user.LastLogin,
		IsActive:           user.IsActive,
		MustChangePassword: user.MustChangePassword,
	}
}
