package types

// Command represents a request from the CLI
type Command struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
}

// Response represents the agent's response
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}
