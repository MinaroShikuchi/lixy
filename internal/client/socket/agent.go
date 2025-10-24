package socket

// ControllerClient extends the base client with controller-specific methods
type AgentController struct {
	*Client
}

func NewAgentController() *AgentController {
	return &AgentController{
		Client: &Client{SocketPath: "/tmp/lixies.sock"},
	}
}
