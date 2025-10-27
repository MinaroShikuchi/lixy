package services

import "github.com/MinaroShikuchi/lixy/internal/store"

type TokenService struct {
	tokenStore *store.TokenStore
}

func NewTokenService(tokenStore *store.TokenStore) *TokenService {
	return &TokenService{
		tokenStore: tokenStore,
	}
}

func (s *TokenService) GetToken() (store.TokenData, error) {
	return s.tokenStore.Get()
}
func (s *TokenService) CreateToken(token, controllerURL string) error {
	return s.tokenStore.Create(token, controllerURL)
}
