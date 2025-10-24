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
	return s.agentStore.ListAgents()
}

func (s *AgentService) CreateAgent(agentID, agentName, ip string, port int) error {
	agent := store.AgentInfo{
		ID:       agentID,
		Name:     agentName,
		IP:       ip,
		Port:     port,
		Status:   "online",
		Metadata: map[string]string{}, // Can be populated from more claims if needed
	}
	return s.agentStore.InsertAgent(agent)
}

func (s *AgentService) DeleteAgent(agentID string) error {
	return s.agentStore.DeleteAgent(agentID)
}
