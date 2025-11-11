package domain

// Agent Command options

type RegisterOptions struct {
	Token      string `json:"token"`
	Controller string `json:"controller"`
}

type UnregisterOptions struct {
}

// Controller Command options
type RegisterAgentOptions struct {
	Expiration int `json:"expiration"`
}

type CreateDeploymentOptions struct {
	Name        string `json:"name"`
	TargetLXC   string `json:"target_lxc"`
	ComposeYAML []byte `json:"compose_yml"`
}

type DeleteDeploymentOptions struct {
	Name string `json:"name"`
}
type GetDeploymentOptions struct {
	Name string `json:"name"`
}

type ReconcileEnvironmentDeploymentsOptions struct {
	Repository  string `json:"repository"`
	Environment string `json:"environment"`
}

// Registry credential command options
type AddRegistryCredentialOptions struct {
	RegistryType string `json:"registry_type"`
	Username     string `json:"username"`
	Token        string `json:"token"`
}

type ListRegistryCredentialsOptions struct {
}

type DeleteRegistryCredentialOptions struct {
	RegistryType string `json:"registry_type"`
}
