package services

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/domain"
	"github.com/MinaroShikuchi/lixy/internal/store"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// SyncResult describes the outcome of syncing a single mapping.
type SyncResult struct {
	MappingName string `json:"mapping_name"`
	Tag         string `json:"tag,omitempty"`
	Updated     bool   `json:"updated"`
	Error       string `json:"error,omitempty"`
}

// HarborService manages Harbor registry configuration, mappings, and sync.
type HarborService struct {
	logger        *slog.Logger
	client        *http.Client
	mappingStore  domain.HarborMappingRepository
	credStore     *store.RegistryCredentialStore
	configStore   *store.ConfigStore
	deploymentSvc *DeploymentService
}

// NewHarborService creates a HarborService with the given dependencies.
func NewHarborService(
	logger *slog.Logger,
	mappingStore domain.HarborMappingRepository,
	credStore *store.RegistryCredentialStore,
	configStore *store.ConfigStore,
	deploymentSvc *DeploymentService,
) *HarborService {
	return &HarborService{
		logger:        logger,
		client:        &http.Client{Timeout: 30 * time.Second},
		mappingStore:  mappingStore,
		credStore:     credStore,
		configStore:   configStore,
		deploymentSvc: deploymentSvc,
	}
}

// --- Config ---

// harborHostname extracts the hostname from the stored Harbor URL, used as the credential key.
// e.g. "https://harbor.example.com" → "harbor.example.com"
func (s *HarborService) harborHostname() (string, error) {
	raw, ok := s.configStore.Get("harbor_url")
	if !ok || raw == "" {
		return "", fmt.Errorf("harbor is not configured")
	}
	u, err := url.Parse(strings.TrimRight(raw, "/"))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid harbor_url %q: %w", raw, err)
	}
	return u.Host, nil
}

// SetConfig stores the Harbor URL and credentials.
// Credentials are stored under the registry hostname so agents can look them
// up by the hostname they extract from image references.
func (s *HarborService) SetConfig(harborURL, username, password string) error {
	if err := s.configStore.Set("harbor_url", harborURL); err != nil {
		return fmt.Errorf("failed to store harbor url: %w", err)
	}
	u, err := url.Parse(strings.TrimRight(harborURL, "/"))
	if err != nil || u.Host == "" {
		return fmt.Errorf("invalid harbor url %q", harborURL)
	}
	if err := s.credStore.StoreCredential(u.Host, username, password); err != nil {
		return fmt.Errorf("failed to store harbor credentials: %w", err)
	}
	return nil
}

// GetConfig returns the Harbor URL and username. configured is false if no URL is stored.
func (s *HarborService) GetConfig() (harborURL, username string, configured bool) {
	u, ok := s.configStore.Get("harbor_url")
	if !ok || u == "" {
		return "", "", false
	}
	hostname, err := s.harborHostname()
	if err == nil {
		cred, err := s.credStore.GetCredential(hostname)
		if err == nil && cred != nil {
			username = cred.Username
		}
	}
	return u, username, true
}

// --- Harbor API client helpers ---

func (s *HarborService) harborURL() (string, error) {
	u, ok := s.configStore.Get("harbor_url")
	if !ok || u == "" {
		return "", fmt.Errorf("harbor is not configured: set harbor_url via POST /api/harbor/config")
	}
	return strings.TrimRight(u, "/"), nil
}

func (s *HarborService) basicAuthHeader() string {
	hostname, err := s.harborHostname()
	if err != nil {
		return ""
	}
	cred, err := s.credStore.GetCredential(hostname)
	if err != nil || cred == nil {
		return ""
	}
	raw := cred.Username + ":" + cred.Token
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

func (s *HarborService) doRequest(path string) (*http.Response, error) {
	base, err := s.harborURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build harbor request: %w", err)
	}
	if auth := s.basicAuthHeader(); auth != "" {
		req.Header.Set("Authorization", auth)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("harbor request failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("harbor API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp, nil
}

// --- Harbor API wrappers ---

// ListProjects returns all projects visible to the configured user.
func (s *HarborService) ListProjects() ([]domain.HarborProject, error) {
	resp, err := s.doRequest("/api/v2.0/projects?page_size=100")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw []struct {
		Name      string `json:"name"`
		RepoCount int    `json:"repo_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode harbor projects: %w", err)
	}
	projects := make([]domain.HarborProject, 0, len(raw))
	for _, r := range raw {
		projects = append(projects, domain.HarborProject{Name: r.Name, RepoCount: r.RepoCount})
	}
	return projects, nil
}

// ListRepositories returns repositories for the given project.
// Harbor returns names as "project/repo"; this method strips the project prefix.
func (s *HarborService) ListRepositories(project string) ([]domain.HarborRepository, error) {
	path := "/api/v2.0/projects/" + url.PathEscape(project) + "/repositories?page_size=100"
	resp, err := s.doRequest(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var raw []struct {
		Name          string `json:"name"`
		ArtifactCount int    `json:"artifact_count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode harbor repositories: %w", err)
	}
	repos := make([]domain.HarborRepository, 0, len(raw))
	prefix := project + "/"
	for _, r := range raw {
		name := strings.TrimPrefix(r.Name, prefix)
		repos = append(repos, domain.HarborRepository{Name: name, ArtifactCount: r.ArtifactCount})
	}
	return repos, nil
}

// ListTags returns tags for the given project/repository.
func (s *HarborService) ListTags(project, repo string) ([]domain.HarborTag, error) {
	path := "/api/v2.0/projects/" + url.PathEscape(project) +
		"/repositories/" + url.PathEscape(repo) +
		"/artifacts?type=IMAGE&page_size=10&with_tag=true"
	resp, err := s.doRequest(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var artifacts []struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&artifacts); err != nil {
		return nil, fmt.Errorf("failed to decode harbor artifacts: %w", err)
	}
	tags := make([]domain.HarborTag, 0)
	for _, a := range artifacts {
		for _, t := range a.Tags {
			tags = append(tags, domain.HarborTag{Name: t.Name})
		}
	}
	return tags, nil
}

// FetchLatestTag returns the first tag of the most recent artifact for a repository.
func (s *HarborService) FetchLatestTag(project, repo string) (string, error) {
	tags, err := s.ListTags(project, repo)
	if err != nil {
		return "", err
	}
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found for %s/%s", project, repo)
	}
	return tags[0].Name, nil
}

// --- Mapping management ---

// CreateMapping stores a new harbor mapping.
func (s *HarborService) CreateMapping(req domain.CreateHarborMappingRequest) (domain.HarborMapping, error) {
	if req.Name == "" || req.Project == "" || req.Repository == "" || req.TargetAgent == "" {
		return domain.HarborMapping{}, fmt.Errorf("name, project, repository, and target_agent are required")
	}
	if _, exists := s.mappingStore.Get(req.Name); exists {
		return domain.HarborMapping{}, fmt.Errorf("mapping with name '%s' already exists", req.Name)
	}
	now := time.Now()
	mapping := domain.HarborMapping{
		ID:              uuid.New().String(),
		Name:            req.Name,
		Project:         req.Project,
		Repository:      req.Repository,
		TargetAgent:     req.TargetAgent,
		ComposeTemplate: req.ComposeTemplate,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.mappingStore.Create(mapping); err != nil {
		return domain.HarborMapping{}, fmt.Errorf("failed to create mapping: %w", err)
	}
	return mapping, nil
}

// ListMappings returns all harbor mappings.
func (s *HarborService) ListMappings() ([]domain.HarborMapping, error) {
	return s.mappingStore.List()
}

// DeleteMapping removes a mapping by name.
func (s *HarborService) DeleteMapping(name string) error {
	if _, exists := s.mappingStore.Get(name); !exists {
		return fmt.Errorf("mapping '%s' not found", name)
	}
	return s.mappingStore.Delete(name)
}

// --- Sync ---

// SyncAll syncs all mappings and returns per-mapping results.
func (s *HarborService) SyncAll() ([]SyncResult, error) {
	mappings, err := s.mappingStore.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list mappings: %w", err)
	}
	results := make([]SyncResult, 0, len(mappings))
	for _, m := range mappings {
		results = append(results, s.syncMapping(m))
	}
	return results, nil
}

// SyncMapping syncs a single named mapping.
func (s *HarborService) SyncMapping(name string) (SyncResult, error) {
	mapping, exists := s.mappingStore.Get(name)
	if !exists {
		return SyncResult{}, fmt.Errorf("mapping '%s' not found", name)
	}
	return s.syncMapping(mapping), nil
}

func (s *HarborService) syncMapping(mapping domain.HarborMapping) SyncResult {
	tag, err := s.FetchLatestTag(mapping.Project, mapping.Repository)
	if err != nil {
		s.logger.Error("harbor sync: failed to fetch latest tag",
			"mapping", mapping.Name, "error", err)
		return SyncResult{MappingName: mapping.Name, Error: err.Error()}
	}

	if tag == mapping.LastSyncedTag {
		return SyncResult{MappingName: mapping.Name, Tag: tag, Updated: false}
	}

	composeYAML, err := s.generateComposeYAML(mapping, tag)
	if err != nil {
		s.logger.Error("harbor sync: failed to generate compose yaml",
			"mapping", mapping.Name, "error", err)
		return SyncResult{MappingName: mapping.Name, Error: err.Error()}
	}

	if err := s.deploymentSvc.CreateOrUpdateDeployment(mapping.Name, mapping.TargetAgent, composeYAML, nil); err != nil {
		s.logger.Error("harbor sync: failed to create/update deployment",
			"mapping", mapping.Name, "error", err)
		return SyncResult{MappingName: mapping.Name, Error: err.Error()}
	}

	now := time.Now()
	mapping.LastSyncedTag = tag
	mapping.LastSyncedAt = &now
	if err := s.mappingStore.Update(mapping); err != nil {
		s.logger.Warn("harbor sync: failed to update mapping last_synced_tag",
			"mapping", mapping.Name, "error", err)
	}

	s.logger.Info("harbor sync: deployment updated", "mapping", mapping.Name, "tag", tag)
	return SyncResult{MappingName: mapping.Name, Tag: tag, Updated: true}
}

// generateComposeYAML produces a docker-compose YAML for the given mapping and tag.
func (s *HarborService) generateComposeYAML(mapping domain.HarborMapping, tag string) ([]byte, error) {
	hostname, err := s.harborHostname()
	if err != nil {
		return nil, err
	}
	image := fmt.Sprintf("%s/%s/%s:%s", hostname, mapping.Project, mapping.Repository, tag)

	if mapping.ComposeTemplate != "" {
		tmpl, err := template.New("compose").Parse(mapping.ComposeTemplate)
		if err != nil {
			return nil, fmt.Errorf("invalid compose_template: %w", err)
		}
		var buf strings.Builder
		data := struct {
			Image       string
			Tag         string
			Project     string
			Repository  string
			MappingName string
		}{
			Image:       image,
			Tag:         tag,
			Project:     mapping.Project,
			Repository:  mapping.Repository,
			MappingName: mapping.Name,
		}
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, fmt.Errorf("failed to execute compose_template: %w", err)
		}
		return []byte(buf.String()), nil
	}

	// Auto-generate a minimal compose file.
	type composeService struct {
		Image   string            `yaml:"image"`
		Restart string            `yaml:"restart"`
		Labels  map[string]string `yaml:"labels,omitempty"`
	}
	type composeFile struct {
		Services map[string]composeService `yaml:"services"`
	}
	compose := composeFile{
		Services: map[string]composeService{
			mapping.Name: {
				Image:   image,
				Restart: "unless-stopped",
				Labels: map[string]string{
					"lixy.managed": "true",
					"lixy.mapping": mapping.Name,
				},
			},
		},
	}
	return yaml.Marshal(compose)
}
