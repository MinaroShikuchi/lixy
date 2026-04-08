package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// AuthService encapsulates JWT authentication state and operations.
type AuthService struct {
	jwtSecret            []byte
	pendingRegistrations map[string]time.Time
	pendingMutex         sync.RWMutex
	usedTokens           map[string]bool
	usedMutex            sync.RWMutex
}

// NewAuthService creates a new AuthService. If jwtSecret is empty, a random
// secret is generated and a warning is logged (development-only behaviour).
func NewAuthService(jwtSecret string) *AuthService {
	if jwtSecret == "" {
		randomKey := make([]byte, 32)
		if _, err := rand.Read(randomKey); err != nil {
			// Fallback: this should never happen in practice.
			panic(fmt.Sprintf("failed to generate random JWT key: %v", err))
		}
		jwtSecret = base64.StdEncoding.EncodeToString(randomKey)
		fmt.Printf("WARNING: No JWT secret provided. Generated random secret: %s\n", jwtSecret)
		fmt.Println("For production, set the LIXY_JWT_SECRET environment variable")
	}

	return &AuthService{
		jwtSecret:            []byte(jwtSecret),
		pendingRegistrations: make(map[string]time.Time),
		usedTokens:           make(map[string]bool),
	}
}

// GenerateRegistrationToken creates a time-limited registration token.
func (s *AuthService) GenerateRegistrationToken(duration time.Duration) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	token := hex.EncodeToString(tokenBytes)
	expirationTime := time.Now().Add(duration * time.Second)

	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expirationTime),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   "registration",
		ID:        token,
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := jwtToken.SignedString(s.jwtSecret)
	if err != nil {
		return "", err
	}

	s.pendingMutex.Lock()
	s.pendingRegistrations[token] = expirationTime
	s.pendingMutex.Unlock()

	return signedToken, nil
}

// GeneratePermanentToken creates a long-lived authentication token for an agent.
func (s *AuthService) GeneratePermanentToken(agentName string) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	expirationTime := time.Now().AddDate(1, 0, 0)

	claims := jwt.MapClaims{
		"sub":        "lixies-agent",
		"name":       agentName,
		"iat":        time.Now().Unix(),
		"exp":        expirationTime.Unix(),
		"iss":        "lixy-controller",
		"token_type": "authentication",
		"permanent":  true,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// GenerateTemporaryToken creates a short-lived authentication token for an agent.
func (s *AuthService) GenerateTemporaryToken(agentName string, duration time.Duration) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}

	expirationTime := time.Now().Add(duration)

	claims := jwt.MapClaims{
		"sub":        "lixies-agent",
		"name":       agentName,
		"iat":        time.Now().Unix(),
		"exp":        expirationTime.Unix(),
		"iss":        "lixy-controller",
		"token_type": "authentication",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

// ValidateAgentToken validates an agent authentication token and returns the agent name.
func (s *AuthService) ValidateAgentToken(token string, remoteIP string) (string, error) {
	if len(s.jwtSecret) == 0 {
		return "", fmt.Errorf("JWT secret not initialized")
	}
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	if !parsedToken.Valid {
		return "", fmt.Errorf("token is not valid")
	}

	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims format")
	}

	if sub, ok := claims["token_type"].(string); !ok || sub != "authentication" {
		return "", fmt.Errorf("invalid claims: not an authentication token")
	}

	agentName, ok := claims["name"].(string)
	if !ok || agentName == "" {
		return "", fmt.Errorf("missing agent ID in token")
	}

	return agentName, nil
}

// ValidateRegistrationToken validates a registration token and ensures it hasn't been used.
func (s *AuthService) ValidateRegistrationToken(tokenString string) (bool, error) {
	if len(s.jwtSecret) == 0 {
		return false, fmt.Errorf("JWT secret not initialized")
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return false, fmt.Errorf("invalid token: %w", err)
	}

	if !token.Valid {
		return false, fmt.Errorf("token validation failed")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, fmt.Errorf("invalid claims format")
	}

	tokenType, hasType := claims["sub"].(string)
	if !hasType || tokenType != "registration" {
		return false, fmt.Errorf("not a registration token")
	}

	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return false, fmt.Errorf("registration token expired")
		}
	} else {
		return false, fmt.Errorf("token missing expiration claim")
	}

	tokenID, hasID := claims["jti"].(string)
	if !hasID || tokenID == "" {
		return false, fmt.Errorf("token missing ID claim")
	}

	s.usedMutex.RLock()
	used := s.usedTokens[tokenID]
	s.usedMutex.RUnlock()

	if used {
		return false, fmt.Errorf("registration token already used")
	}

	s.usedMutex.Lock()
	s.usedTokens[tokenID] = true
	s.usedMutex.Unlock()

	return true, nil
}
