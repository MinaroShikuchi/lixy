package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/golang-jwt/jwt/v5"
)

// PullTokenClaims represents JWT claims for pull tokens
type PullTokenClaims struct {
	AgentName string `json:"agent_name"`
	Registry  string `json:"registry"`
	jwt.RegisteredClaims
}

// PullTokenResponse represents the response when issuing a pull token
type PullTokenResponse struct {
	Token     string    `json:"token"`
	Username  string    `json:"username"`
	Registry  string    `json:"registry"`
	ExpiresIn int       `json:"expires_in"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PullTokenService manages pull token generation and validation
type PullTokenService struct {
	logger    *slog.Logger
	credStore *store.RegistryCredentialStore
	jwtSecret []byte
	tokenTTL  time.Duration // 15 minutes
}

// NewPullTokenService creates a new pull token service
func NewPullTokenService(
	logger *slog.Logger,
	credStore *store.RegistryCredentialStore,
	jwtSecret []byte,
) *PullTokenService {
	return &PullTokenService{
		logger:    logger,
		credStore: credStore,
		jwtSecret: jwtSecret,
		tokenTTL:  15 * time.Minute, // 15 minutes as per requirements
	}
}

// GeneratePullToken generates a temporary pull token for an agent
// All registries (including GHCR) now use stored credentials from the registry credential store
func (s *PullTokenService) GeneratePullToken(agentName, registry string) (*PullTokenResponse, error) {
	// Validate inputs
	if agentName == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}
	if registry == "" {
		return nil, fmt.Errorf("registry cannot be empty")
	}

	// Retrieve stored credentials for the registry
	cred, err := s.credStore.GetCredential(registry)
	if err != nil {
		s.logger.Error("Failed to retrieve registry credentials",
			"registry", registry,
			"error", err)
		return nil, fmt.Errorf("failed to get credentials for registry %s: %w", registry, err)
	}

	// Create JWT claims for audit/tracking purposes
	expiresAt := time.Now().Add(s.tokenTTL)
	tokenID, err := generateTokenID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	claims := &PullTokenClaims{
		AgentName: agentName,
		Registry:  registry,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "pull-token",
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "lixy-controller",
			ID:        tokenID,
		},
	}

	// Sign token (for audit trail, not used for authentication)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	_, err = token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	// Log token issuance for audit
	s.logger.Info("Pull token issued",
		"agent", agentName,
		"registry", registry,
		"username", cred.Username,
		"expires_at", expiresAt,
		"token_id", tokenID)

	// Determine registry URL based on type
	registryURL := s.getRegistryURL(registry)

	// Return response with actual registry token
	return &PullTokenResponse{
		Token:     cred.Token,
		Username:  cred.Username,
		Registry:  registryURL,
		ExpiresIn: int(s.tokenTTL.Seconds()),
		ExpiresAt: expiresAt,
	}, nil
}

// ValidatePullToken validates a pull token and returns its claims
// Note: This is primarily for audit purposes since we return the actual registry token
func (s *PullTokenService) ValidatePullToken(tokenString string) (*PullTokenClaims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &PullTokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.jwtSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	// Extract claims
	claims, ok := token.Claims.(*PullTokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify token hasn't expired
	if claims.ExpiresAt != nil && claims.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("token has expired")
	}

	// Verify this is a pull token
	if claims.Subject != "pull-token" {
		return nil, fmt.Errorf("not a pull token")
	}

	return claims, nil
}

// GetRegistryCredentials retrieves credentials for a registry
func (s *PullTokenService) GetRegistryCredentials(registry string) (*store.RegistryCredential, error) {
	return s.credStore.GetCredential(registry)
}

// getRegistryURL returns the registry URL based on registry type
func (s *PullTokenService) getRegistryURL(registryType string) string {
	switch registryType {
	case "ghcr":
		return "ghcr.io"
	default:
		return registryType
	}
}

// generateTokenID generates a unique token ID for tracking
func generateTokenID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// SetTokenTTL allows customizing the token TTL (useful for testing)
func (s *PullTokenService) SetTokenTTL(ttl time.Duration) {
	s.tokenTTL = ttl
}

// GetTokenTTL returns the current token TTL
func (s *PullTokenService) GetTokenTTL() time.Duration {
	return s.tokenTTL
}
