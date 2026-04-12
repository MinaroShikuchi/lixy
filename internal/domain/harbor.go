package domain

import "time"

// HarborMapping represents a mapping from a Harbor repository to an LXC agent.
type HarborMapping struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Project         string     `json:"project"`
	Repository      string     `json:"repository"`
	TargetAgent     string     `json:"target_agent"`
	ComposeTemplate string     `json:"compose_template"`
	LastSyncedTag   string     `json:"last_synced_tag"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// CreateHarborMappingRequest is the HTTP request body for POST /api/harbor/mappings.
type CreateHarborMappingRequest struct {
	Name            string `json:"name"`
	Project         string `json:"project"`
	Repository      string `json:"repository"`
	TargetAgent     string `json:"target_agent"`
	ComposeTemplate string `json:"compose_template,omitempty"`
}

// HarborConfigRequest is the HTTP request body for POST /api/harbor/config.
type HarborConfigRequest struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// HarborConfigResponse is returned by GET /api/harbor/config.
type HarborConfigResponse struct {
	URL      string `json:"url"`
	Username string `json:"username"`
}

// HarborProject is a project returned by the Harbor API.
type HarborProject struct {
	Name      string `json:"name"`
	RepoCount int    `json:"repo_count"`
}

// HarborRepository is a repository returned by the Harbor API.
type HarborRepository struct {
	Name          string `json:"name"`
	ArtifactCount int    `json:"artifact_count"`
}

// HarborTag is a tag returned by the Harbor API.
type HarborTag struct {
	Name string `json:"name"`
}

// HarborMappingRepository defines the persistence interface for harbor mappings.
type HarborMappingRepository interface {
	Get(name string) (HarborMapping, bool)
	List() ([]HarborMapping, error)
	Create(mapping HarborMapping) error
	Update(mapping HarborMapping) error
	Delete(name string) error
}
