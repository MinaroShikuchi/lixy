package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	userService *services.UserService
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(userService *services.UserService) *AuthHandler {
	return &AuthHandler{
		userService: userService,
	}
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req domain.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	// Authenticate user
	user, err := h.userService.Authenticate(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Generate token
	token, err := h.userService.GenerateToken(user)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Prepare response
	expiresIn := int(services.UserTokenExpiry.Seconds())
	response := domain.LoginResponse{
		Token:     token,
		ExpiresIn: expiresIn,
		ExpiresAt: time.Now().Add(services.UserTokenExpiry),
		User:      *h.userService.ToUserInfo(user),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// For JWT tokens, logout is handled client-side by removing the token
	// In the future, we could implement token blacklisting if needed
	w.WriteHeader(http.StatusNoContent)
}

// GetCurrentUser returns information about the currently authenticated user
func (h *AuthHandler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID, ok := r.Context().Value(domain.UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.userService.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	userInfo := h.userService.ToUserInfo(user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// ChangePassword handles password change for the current user
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	userID, ok := r.Context().Value(domain.UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var req domain.ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current password and new password are required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Change password
	if err := h.userService.ChangePassword(userID, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUsers returns all users (admin only)
func (h *AuthHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	users, err := h.userService.ListUsers()
	if err != nil {
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}

	// Convert to UserInfo to avoid exposing password hashes
	userInfos := make([]*domain.UserInfo, len(users))
	for i, user := range users {
		userInfos[i] = h.userService.ToUserInfo(user)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfos)
}

// GetUser returns a specific user (admin only)
func (h *AuthHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract user ID from URL path
	idStr := r.URL.Path[len("/api/users/"):]
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	user, err := h.userService.GetUser(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	userInfo := h.userService.ToUserInfo(user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// CreateUser creates a new user (admin only)
func (h *AuthHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	var req domain.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "Username and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	if req.Role == "" {
		req.Role = "user" // Default role
	}

	if req.Role != "admin" && req.Role != "user" {
		http.Error(w, "Role must be 'admin' or 'user'", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := h.userService.CreateUser(&req)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Error(w, "Username already exists", http.StatusConflict)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to create user: %v", err), http.StatusInternalServerError)
		return
	}

	userInfo := h.userService.ToUserInfo(user)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(userInfo)
}

// UpdateUser updates a user (admin only)
func (h *AuthHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract user ID from URL path
	idStr := r.URL.Path[len("/api/users/"):]
	// Remove any trailing path segments (e.g., /password)
	if idx := strings.Index(idStr, "/"); idx != -1 {
		idStr = idStr[:idx]
	}
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req domain.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate role if provided
	if req.Role != nil && *req.Role != "admin" && *req.Role != "user" {
		http.Error(w, "Role must be 'admin' or 'user'", http.StatusBadRequest)
		return
	}

	// Update user
	user, err := h.userService.UpdateUser(userID, &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update user: %v", err), http.StatusInternalServerError)
		return
	}

	userInfo := h.userService.ToUserInfo(user)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(userInfo)
}

// ResetPassword resets a user's password (admin only)
func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract user ID from URL path (e.g., /api/users/1/password)
	path := r.URL.Path[len("/api/users/"):]
	idStr := path[:strings.Index(path, "/")]
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var req domain.ResetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.NewPassword == "" {
		http.Error(w, "New password is required", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		http.Error(w, "New password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Reset password
	if err := h.userService.ResetPassword(userID, &req); err != nil {
		http.Error(w, fmt.Sprintf("Failed to reset password: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteUser deletes a user (admin only)
func (h *AuthHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	// Check if user is admin
	role, ok := r.Context().Value(domain.UserRoleKey).(string)
	if !ok || role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Extract user ID from URL path
	idStr := r.URL.Path[len("/api/users/"):]
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Prevent deleting yourself
	currentUserID, ok := r.Context().Value(domain.UserIDKey).(int)
	if ok && currentUserID == userID {
		http.Error(w, "Cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Delete user
	if err := h.userService.DeleteUser(userID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete user: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleUsers routes user list/create requests based on HTTP method
func (h *AuthHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListUsers(w, r)
	case http.MethodPost:
		h.CreateUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUserByID routes user get/update/delete requests based on HTTP method
func (h *AuthHandler) HandleUserByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.GetUser(w, r)
	case http.MethodPatch:
		h.UpdateUser(w, r)
	case http.MethodDelete:
		h.DeleteUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
