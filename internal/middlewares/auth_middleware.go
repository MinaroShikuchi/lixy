package middlewares

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/MinaroShikuchi/lixy/internal/services"
)

// Middleware to authenticate API requests
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
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

		// Get client's IP address
		remoteIP := r.Header.Get("X-Forwarded-For")
		if remoteIP == "" {
			remoteIP = r.RemoteAddr
		}

		// Validate token and identify agent
		agentID, err := services.ValidateAgentToken(token, remoteIP)
		if err != nil {
			log.Printf("Unauthorized access attempt from IP %s: %v", remoteIP, err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Add agent ID to request context for handlers to use
		ctx := context.WithValue(r.Context(), "agent_id", agentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
