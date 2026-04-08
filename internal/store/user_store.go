package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
)

// Compile-time check that UserStore implements domain.UserRepository
var _ domain.UserRepository = (*UserStore)(nil)

// UserStore handles database operations for users
type UserStore struct {
	db *sql.DB
}

// NewUserStore creates a new user store
func NewUserStore(db *sql.DB) (*UserStore, error) {
	store := &UserStore{db: db}

	// Create users table if it doesn't exist
	if err := store.createTable(); err != nil {
		return nil, fmt.Errorf("failed to create users table: %w", err)
	}

	return store, nil
}

// createTable creates the users table
func (s *UserStore) createTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		email TEXT UNIQUE,
		role TEXT NOT NULL DEFAULT 'user',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME,
		is_active BOOLEAN DEFAULT 1,
		must_change_password BOOLEAN DEFAULT 0
	);
	
	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
	`

	_, err := s.db.Exec(query)
	return err
}

// Create creates a new user
func (s *UserStore) Create(user *domain.User) error {
	query := `
	INSERT INTO users (username, password_hash, email, role, is_active, must_change_password)
	VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := s.db.Exec(query,
		user.Username,
		user.PasswordHash,
		user.Email,
		user.Role,
		user.IsActive,
		user.MustChangePassword,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get user ID: %w", err)
	}

	user.ID = int(id)
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	return nil
}

// GetByID retrieves a user by ID
func (s *UserStore) GetByID(id int) (*domain.User, error) {
	query := `
	SELECT id, username, password_hash, email, role, created_at, updated_at, last_login, is_active, must_change_password
	FROM users
	WHERE id = ?
	`

	user := &domain.User{}
	var lastLogin sql.NullTime

	err := s.db.QueryRow(query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
		&user.IsActive,
		&user.MustChangePassword,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetByUsername retrieves a user by username
func (s *UserStore) GetByUsername(username string) (*domain.User, error) {
	query := `
	SELECT id, username, password_hash, email, role, created_at, updated_at, last_login, is_active, must_change_password
	FROM users
	WHERE username = ?
	`

	user := &domain.User{}
	var lastLogin sql.NullTime

	err := s.db.QueryRow(query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
		&user.IsActive,
		&user.MustChangePassword,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetByEmail retrieves a user by email
func (s *UserStore) GetByEmail(email string) (*domain.User, error) {
	query := `
	SELECT id, username, password_hash, email, role, created_at, updated_at, last_login, is_active, must_change_password
	FROM users
	WHERE email = ?
	`

	user := &domain.User{}
	var lastLogin sql.NullTime

	err := s.db.QueryRow(query, email).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&lastLogin,
		&user.IsActive,
		&user.MustChangePassword,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// List retrieves all users
func (s *UserStore) List() ([]*domain.User, error) {
	query := `
	SELECT id, username, password_hash, email, role, created_at, updated_at, last_login, is_active, must_change_password
	FROM users
	ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
			&lastLogin,
			&user.IsActive,
			&user.MustChangePassword,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// GetByRole retrieves users by role
func (s *UserStore) GetByRole(role string) ([]*domain.User, error) {
	query := `
	SELECT id, username, password_hash, email, role, created_at, updated_at, last_login, is_active, must_change_password
	FROM users
	WHERE role = ?
	ORDER BY created_at DESC
	`

	rows, err := s.db.Query(query, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by role: %w", err)
	}
	defer rows.Close()

	var users []*domain.User
	for rows.Next() {
		user := &domain.User{}
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.PasswordHash,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
			&lastLogin,
			&user.IsActive,
			&user.MustChangePassword,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// Update updates a user
func (s *UserStore) Update(user *domain.User) error {
	query := `
	UPDATE users
	SET email = ?, role = ?, updated_at = ?, is_active = ?, must_change_password = ?
	WHERE id = ?
	`

	user.UpdatedAt = time.Now()

	_, err := s.db.Exec(query,
		user.Email,
		user.Role,
		user.UpdatedAt,
		user.IsActive,
		user.MustChangePassword,
		user.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

// UpdatePassword updates a user's password hash
func (s *UserStore) UpdatePassword(userID int, passwordHash string) error {
	query := `
	UPDATE users
	SET password_hash = ?, updated_at = ?, must_change_password = 0
	WHERE id = ?
	`

	_, err := s.db.Exec(query, passwordHash, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// UpdateLastLogin updates the last login timestamp
func (s *UserStore) UpdateLastLogin(userID int) error {
	query := `
	UPDATE users
	SET last_login = ?
	WHERE id = ?
	`

	_, err := s.db.Exec(query, time.Now(), userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}

	return nil
}

// Delete deletes a user
func (s *UserStore) Delete(id int) error {
	query := `DELETE FROM users WHERE id = ?`

	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// Count returns the total number of users
func (s *UserStore) Count() (int, error) {
	query := `SELECT COUNT(*) FROM users`

	var count int
	err := s.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// CountByRole returns the number of users with a specific role
func (s *UserStore) CountByRole(role string) (int, error) {
	query := `SELECT COUNT(*) FROM users WHERE role = ?`

	var count int
	err := s.db.QueryRow(query, role).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users by role: %w", err)
	}

	return count, nil
}
