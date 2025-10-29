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
