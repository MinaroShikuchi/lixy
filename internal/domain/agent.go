package domain

import "time"

// AgentInfo represents the registered agent information
type AgentInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	IP           string            `json:"ip"`
	Port         int               `json:"port"`
	Capabilities map[string]string `json:"capabilities"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
	Status       string            `json:"status"` // "online", "offline", "unreachable"
	Metadata     map[string]string `json:"metadata"`
}

// SystemInfo holds agent environment information
type SystemInfo struct {
	Version  string `json:"version"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Port     int    `json:"port"`
}

// RegistrationResponse is the struct for registration response
type RegistrationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

// RegistrationRequest is the struct for decoding registration request
type RegistrationRequest struct {
	Hostname string `json:"hostname"`
	Token    string `json:"token"`
}

// UnregistrationRequest is the struct for decoding unregistration request
type UnregistrationRequest struct {
}

// SaveAgentRequest is the struct for decoding save agent request
type SaveAgentRequest struct {
	Version string `json:"version"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

// SaveAgentResponse is the struct for decoding save agent response
type SaveAgentResponse struct {
	Success bool              `json:"success"`
	Message string            `json:"message,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
}

// AgentRepository defines the persistence interface for agents
type AgentRepository interface {
	Get(name string) (AgentInfo, bool)
	List() []AgentInfo
	Create(agent AgentInfo) error
	Upsert(agent AgentInfo) error
	Delete(name string) error
}
