
# Lixy - GitOps Controller-Agent for LXC Container Management

Lixy is a lightweight GitOps-inspired system for managing container deployments across LXC containers in Proxmox environments. It provides a centralized controller with distributed agents that enable consistent, declarative management of containerized applications.

<!-- ![Lixy Architecture](docs/architecture.png) -->

## Features

- **GitOps Workflow**: Declarative container management with Git as the single source of truth
- **Agent-Based Architecture**: Central controller with lightweight agents (lixies) running on target LXC containers
- **Docker Compose Integration**: Deploy containerized applications using familiar Docker Compose files
- **Health Monitoring**: Automated health checks to ensure all agents are operational
- **Token-Based Authentication**: Secure communication between controllers and agents
- **Unix Socket & HTTP APIs**: Multiple communication interfaces for flexibility
- **CLI Tool**: Powerful command-line interface for system management

## Future Feature

- Deployment
    - get deployment => ok
    - create deployment => 
    - update deployment => 
    - delete deployment => 
    - retrieve deployment from controller and parse it to execute the tasks =>
- lixies should have a verbose mode where they are logging the incoming request and the scheduling of their tasks
- lixies should have a task list to execute so they can do it one by one 
    docker compose pull  => ok 
    docker compose up -d => ok

- implement a key value / sqlite to store the version of the container in a deployment

- lixies cli should connect through the socket to lixies agent to get the agent info
- same type for input / ouput in the socket 
    - lixies // icon validarted
    - lixy // icon cross 

## Architecture

You can generate the internal graph import with:
```
godepgraph -s ./cmd/lixy/cli | dot -Tpng -o .graph/lixy_cli_v3.png
godepgraph -s ./cmd/lixies | dot -Tpng -o .graph/lixies_client_v9.png
```

Lixy follows a controller-agent architecture:

```
┌─────────────┐          ┌─────────────┐
│ Controller  │          │    Agent    │
│    CLI      │          │     CLI     │
└──────┬──────┘          └──────┬──────┘
       │ UNIX Socket            │ UNIX Socket
┌──────▼──────┐  HTTP    ┌──────▼──────┐
│ Controller  ◄─────────►│    Agent    │
│   Server    │          │   Server    │
└─────────────┘          └─────────────┘
```

```
┌─────────────────┐                 ┌─────────────────┐
│                 │                 │                 │
│  Lixy           │◄───Socket/HTTP──┤  CLI Tool       │
│  Controller     │                 │                 │
│                 │                 └─────────────────┘
└───────┬─────────┘
        │
        │ HTTP
        │
┌───────▼─────────┐     ┌─────────────────┐     ┌─────────────────┐
│                 │     │                 │     │                 │
│  Lixies Agent   │     │  Lixies Agent   │     │  Lixies Agent   │
│  (LXC 101)      │     │  (LXC 102)      │     │  (LXC 103)      │
│                 │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
        ▼                       ▼                       ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Containers     │     │  Containers     │     │  Containers     │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

- **Lixy Controller**: Centralized management service that coordinates deployments
- **Lixies Agents**: Lightweight services running on each LXC container
- **CLI**: Command-line interface for interacting with the controller

## Getting Started

### Prerequisites

- Proxmox VE 7.0+
- LXC containers with Docker installed
- Go 1.19+ (for building from source)
- SQLite3
- Git

### Installation

#### Controller (Lixy)

```bash
# Clone the repository
git clone https://github.com/MinaroShikuchi/lixy.git
cd lixy

# Build the controller
go build -o lixy ./cmd/lixy

# Set the JWT secret for token signing
```
export LIXY_JWT_SECRET=$(openssl rand -base64 32)
export LIXY_JWT_SECRET=nh91W2iL0wtnxy59EmJ98V4hM7CwGZg6AtjNy/oki9w=
export GOPATH=/Users/username/go
export PATH=$GOPATH/bin:$PATH
export PATH="$PATH:/Applications/Docker.app/Contents/Resources/bin/"

```
# Start the controller
./lixy
```

#### Agent (Lixies)

On each LXC container where you want to run the agent:

```bash
# Build the agent
go build -o lixies ./cmd/lixies

# Register with the controller (get a token from controller first)
./lixies register --token <YOUR_REGISTRATION_TOKEN> --controller http://controller-ip:8080 --name lxc-101

# Start the agent service
sudo cp lixies.service /etc/systemd/system/
sudo systemctl enable lixies
sudo systemctl start lixies
```

#### CLI Tool

```bash
# Build the CLI tool
go build -o lixy-cli ./cmd/cli

# Test the connection
./lixy-cli get targets
```

### Usage Examples

#### Deploying a Container

```bash
# Create a deployment using Docker Compose
lixy-cli create deployment myapp --target-lxc 101 --compose-file ./docker-compose.yml
```

#### Viewing Deployments

```bash
# List all deployments
lixy-cli get deployments

# Get details of a specific deployment
lixy-cli get deployment myapp
```

#### Managing Targets (LXC Containers)

```bash
# List all target LXC containers
lixy-cli get targets

# Check health status of a specific target
lixy-cli health 101
```

#### Updating a Deployment

```bash
# Update an existing deployment
lixy-cli update deployment myapp --compose-file ./new-docker-compose.yml
```

## Configuration

### Controller Configuration

The Lixy controller can be configured via environment variables:

```bash
# Required
export LIXY_JWT_SECRET="your-secure-jwt-secret"  # Used for token signing

# Optional
export LIXY_DB_PATH="/var/lib/lixy/db"           # Database path
export LIXY_LOG_LEVEL="info"                     # Log level (debug, info, warn, error)
export LIXY_PORT="8080"                          # HTTP port for the API
```

### Agent Configuration

Lixies agents can be configured via environment variables or command-line flags:

```bash
# Environment variables
export LIXIES_CONTROLLER="http://controller-ip:8080"  # Controller URL
export LIXIES_LOG_LEVEL="info"                        # Log level (debug, info, warn, error)
export LIXIES_WORK_DIR="/var/lib/lixies"              # Working directory
```

## Project Structure

```
lixy/
├── README.md
├── cmd/                      # Application entry points
│   ├── lixy/                 # Controller (Lixy) binary entrypoints
│   │   └── cli/              # CLI controller command set
│   └── lixies/               # Agent (lixies) binary entrypoints
│       └── cli/              # CLI agent command set
└── internal/                 # Private application code (not imported by other projects)
    ├── agent/                # Agent server, routers and handlers
    ├── auth/                 # Authentication logic and token management
    ├── client/               # Internal client factory and helpers
    ├── controller/           # Controller server, routers and handlers (service layer TODO)
    ├── domain/               # Domain interfaces and core types
    ├── middlewares/          # HTTP middleware (logging, auth, etc.)
    ├── services/             # Business logic / service implementations
    └── store/                # Persistence implementations (sqlite, token/agent stores)
```

## Security

Lixy uses a token-based authentication system:

1. The controller generates registration tokens for new agents
2. Agents use registration tokens to securely join the system
3. Upon successful registration, agents receive permanent authentication tokens
4. All subsequent API calls use these tokens for authentication
5. Communication between components is encrypted using TLS

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgements

- The GitOps community for inspiration and best practices
- The Go community for excellent libraries and tools
- Proxmox for their powerful virtualization platform

---

*This README follows the best practices and templates for Go projects.* 