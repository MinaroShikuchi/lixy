package middlewares

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/services"
)

// AuthMiddleware is a unified middleware that accepts both user and agent tokens
// It tries to validate as a user token first, then falls back to agent token
// This allows endpoints to be accessed by both web users and agents
func AuthMiddleware(userService *services.UserService) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			// Extract token from "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
				return
			}

			token := parts[1]
			ctx := r.Context()

			// Try to validate as user token first
			if userService != nil {
				if userClaims, err := userService.ValidateToken(token); err == nil {
					// Valid user token - add user info to context
					ctx = context.WithValue(ctx, "user_id", userClaims.UserID)
					ctx = context.WithValue(ctx, "username", userClaims.Username)
					ctx = context.WithValue(ctx, "user_role", userClaims.Role)
					ctx = context.WithValue(ctx, "token_type", "user")
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			// If not a user token, try to validate as agent token
			// Get client's IP address for agent validation
			remoteIP := r.Header.Get("X-Forwarded-For")
			if remoteIP == "" {
				remoteIP = r.RemoteAddr
			}

			agentName, err := services.ValidateAgentToken(token, remoteIP)
			if err != nil {
				fmt.Printf("Token validation failed: %v\n", err)
				http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Valid agent token - add agent info to context
			ctx = context.WithValue(ctx, domain.AgentNameKey, agentName)
			ctx = context.WithValue(ctx, "token_type", "agent")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
