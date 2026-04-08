package services

import (
	"github.com/MinaroShikuchi/lixy/internal/domain"
)

type TokenService struct {
	tokenStore domain.TokenRepository
}

func NewTokenService(tokenStore domain.TokenRepository) *TokenService {
	return &TokenService{
		tokenStore: tokenStore,
	}
}

func (s *TokenService) GetToken() (domain.TokenData, error) {
	return s.tokenStore.Get()
}
func (s *TokenService) CreateToken(token, controllerURL string) error {
	return s.tokenStore.Create(token, controllerURL)
}
