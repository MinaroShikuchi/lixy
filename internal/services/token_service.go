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

func (s *TokenService) GetToken(agentID string) (store.TokenData, error) {
	return s.tokenStore.GetToken(agentID)
}
func (s *TokenService) CreateToken(agentID, token, controllerURL string) error {
	return s.tokenStore.StoreToken(agentID, token, controllerURL)
}
