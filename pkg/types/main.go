package types

type DeploymentRequest struct {
	Name        string            `json:"name"`
	ComposeYAML []byte            `json:"composeYAML"`
	EnvVars     map[string]string `json:"envVars,omitempty"`
}

type DeploymentResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Output  string `json:"output,omitempty"`
}
