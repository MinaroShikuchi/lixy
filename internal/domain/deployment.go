package domain

// DeploymentInfo represents information about a deployment with docker compose
type DeploymentInfo struct {
	ID          string
	Name        string
	TargetLXC   string
	ComposeYAML []byte
	Status      string
	EnvVars     map[string]string
}

// DeploymentRequest represents a request to deploy an application
type DeploymentRequest struct {
	Name        string            `json:"name"`
	ComposeYAML []byte            `json:"composeYAML"`
	EnvVars     map[string]string `json:"envVars,omitempty"`
}

// DeploymentResponse represents the response from a deployment operation
type DeploymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}

// DeploymentDto is the data transfer object for deployments
type DeploymentDto struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	TargetLXC    string            `json:"target_lxc"`
	TargetAgents []string          `json:"target_agents"`
	Status       string            `json:"status"`
	ComposeYAML  string            `json:"compose_yaml"`
	ComposeFile  string            `json:"compose_file"`
	EnvVars      map[string]string `json:"env_vars,omitempty"`
}

// DeploymentStatusUpdate represents a status update for a deployment
type DeploymentStatusUpdate struct {
	Status string `json:"status"`
}

// CreateDeploymentRequest represents a request to create a new deployment
type CreateDeploymentRequest struct {
	Name        string            `json:"name"`
	ComposeYAML []byte            `json:"compose_yaml"`
	TargetLXC   string            `json:"target_lxc"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
}

// DeploymentRepository defines the persistence interface for deployments
type DeploymentRepository interface {
	Get(name string) (DeploymentInfo, bool)
	List() ([]DeploymentInfo, error)
	Create(deployment DeploymentInfo) error
	CreateOrUpdate(deployment DeploymentInfo) error
	Update(deployment DeploymentInfo) error
	Delete(name string) error
}
