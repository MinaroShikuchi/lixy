package services

import "github.com/MinaroShikuchi/lixy/internal/store"

type AgentService struct {
	agentStore *store.AgentStore
}

func NewAgentService(agentStore *store.AgentStore) *AgentService {
	return &AgentService{
		agentStore: agentStore,
	}
}

func (s *AgentService) ListAgents() []store.AgentInfo {
	return s.agentStore.List()
}

func (s *AgentService) CreateAgent(agentName, ip string, port int) error {
	agent := store.AgentInfo{
		Name:     agentName,
		IP:       ip,
		Port:     port,
		Status:   "online",
		Metadata: map[string]string{}, // Can be populated from more claims if needed
	}
	return s.agentStore.Create(agent)
}

func (s *AgentService) DeleteAgent(agentName string) error {
	return s.agentStore.Delete(agentName)
}
