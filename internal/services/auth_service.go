package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// SetAgentStore sets the agent store for the auth package

var (
	jwtSecret                 []byte
	pendingRegistrations      = make(map[string]time.Time)
	pendingRegistrationsMutex sync.RWMutex
	usedTokens                = make(map[string]bool)
	usedTokensMutex           sync.RWMutex
)

func InitializeAuth() error {
	// Option 1: Load from environment variable (recommended)
	secretKey := os.Getenv("LIXY_JWT_SECRET")
	if secretKey == "" {
		// For development only: generate a random key if not provided
		// In production, you should always provide a stable secret
		randomKey := make([]byte, 32)
		if _, err := rand.Read(randomKey); err != nil {
			return fmt.Errorf("failed to generate random key: %w", err)
		}
		secretKey = base64.StdEncoding.EncodeToString(randomKey)
		fmt.Printf("WARNING: No JWT secret provided. Generated random secret: %s\n", secretKey)
		fmt.Println("For production, set the LIXY_JWT_SECRET environment variable")
	}

	jwtSecret = []byte(secretKey)
	return nil
}

// In the lixy controller
func GenerateRegistrationToken(duration time.Duration) (string, error) {
	// Check if the secret has been initialized
	if len(jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	// Implementation for generating secure registration tokens
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	token := hex.EncodeToString(tokenBytes)
	expirationTime := time.Now().Add(duration * time.Second)
	//debug log

	log.Printf("Generated registration token %s expiring at %s", token, duration)
	log.Printf("Generated registration token %s expiring at %s", token, expirationTime.String())
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "registration",
		ID:        token,
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := jwtToken.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}

	// Store in pending registrations map
	pendingRegistrationsMutex.Lock()
	pendingRegistrations[token] = expirationTime
	pendingRegistrationsMutex.Unlock()

	return signedToken, nil
}

// GeneratePermanentToken creates a long-lived authentication token for an agent
func GeneratePermanentToken(agentID, agentName string) (string, error) {
	// Check if the secret has been initialized
	if len(jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	// Define token expiration (1 year - long-lived but not truly permanent)
	// Setting a very long expiration rather than no expiration is a security best practice
	expirationTime := time.Now().AddDate(1, 0, 0)

	// Create custom claims with agent information
	claims := jwt.MapClaims{
		"sub":        agentID,               // Subject (the entity this token represents)
		"name":       agentName,             // Agent name for reference
		"agent_id":   agentID,               // Duplicate for explicitness
		"iat":        time.Now().Unix(),     // Issued at timestamp
		"exp":        expirationTime.Unix(), // Expiration time
		"iss":        "lixy-controller",     // Issuer (your controller)
		"token_type": "authentication",      // Token purpose (differentiates from registration tokens)
		"permanent":  true,                  // Indicates this is a permanent authentication token
	}

	// Create the JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func ValidateAgentToken(token string, remoteIP string) (string, error) {
	// Implementation for token validation
	if len(jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("Token from IP %s has unexpected signing method: %v", remoteIP, token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		log.Printf("Token validation error from IP %s: %v", remoteIP, err)
		return "", fmt.Errorf("invalid token: %w", err)
	}

	if !parsedToken.Valid {

		return "", fmt.Errorf("token is not valid")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims format")
	}

	// Check if token is for authentication (not registration)
	if sub, ok := claims["token_type"].(string); !ok || sub != "authentication" {
		return "", fmt.Errorf("invalid claims: not an authentication token")
	}

	// Extract and return agent ID from claims
	agentID, ok := claims["sub"].(string)
	if !ok || agentID == "" {
		return "", fmt.Errorf("missing agent ID in token")
	}

	return agentID, nil
}

// ValidateRegistrationToken validates a registration token and ensures it hasn't been used
func ValidateRegistrationToken(tokenString string) (bool, error) {
	// Check if JWT secret is initialized
	if len(jwtSecret) == 0 {
		return false, fmt.Errorf("JWT secret not initialized")
	}
	log.Printf("Validating registration token")

	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil {
		return false, fmt.Errorf("invalid token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return false, fmt.Errorf("token validation failed")
	}

	// Cast the claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("invalid claims format")
	}

	// Verify this is a registration token
	tokenType, hasType := claims["sub"].(string)
	if !hasType || tokenType != "registration" {
		log.Printf("Token is not a registration token")
		return false, fmt.Errorf("not a registration token")
	}

	// Verify expiration manually (the jwt library also checks this,
	// but we want to provide a clearer error message)
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return false, fmt.Errorf("registration token expired")
		}
	} else {
		log.Printf("Token missing expiration claim")
		return false, fmt.Errorf("token missing expiration claim")
	}

	// Get token ID from claims
	tokenID, hasID := claims["jti"].(string)
	if !hasID || tokenID == "" {
		return false, fmt.Errorf("token missing ID claim")
	}

	// Check if token has been used
	usedTokensMutex.RLock()
	used := usedTokens[tokenID]
	usedTokensMutex.RUnlock()

	if used {
		return false, fmt.Errorf("registration token already used")
	}

	// Mark token as used
	usedTokensMutex.Lock()
	usedTokens[tokenID] = true
	usedTokensMutex.Unlock()

	// Periodically clean up expired used tokens (could be a separate goroutine)
	// go cleanupExpiredTokens()

	return true, nil
}
