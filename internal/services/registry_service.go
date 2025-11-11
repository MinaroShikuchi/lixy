package services

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
)

// RegistryService handles container registry operations
type RegistryService struct {
	logger          *slog.Logger
	client          *http.Client
	credentialStore *store.RegistryCredentialStore
}

// NewRegistryService creates a new registry service
func NewRegistryService(logger *slog.Logger, credentialStore *store.RegistryCredentialStore) *RegistryService {
	return &RegistryService{
		logger: logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		credentialStore: credentialStore,
	}
}

// ImageTag represents a container image tag
type ImageTag struct {
	Name      string    `json:"name"`
	Digest    string    `json:"digest,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

// GitHubPackageResponse represents GitHub Container Registry package response
type GitHubPackageResponse struct {
	Tags []string `json:"tags"`
}

// DockerRegistryTagsResponse represents Docker Registry API v2 tags response
type DockerRegistryTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

// FetchGitHubContainerTags fetches available tags from GitHub Container Registry
func (rs *RegistryService) FetchGitHubContainerTags(image, token string) ([]string, error) {
	// Parse image name: ghcr.io/owner/repo or ghcr.io/owner/repo/image
	parts := strings.Split(image, "/")
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid GitHub container image format: %s (expected ghcr.io/owner/repo)", image)
	}

	// Remove ghcr.io prefix
	owner := parts[1]
	packageName := strings.Join(parts[2:], "/")

	rs.logger.Info("Fetching tags from GitHub Container Registry",
		"owner", owner,
		"package", packageName)

	// GitHub Container Registry API endpoint
	// https://docs.github.com/en/rest/packages
	url := fmt.Sprintf("https://api.github.com/users/%s/packages/container/%s/versions",
		owner,
		strings.ReplaceAll(packageName, "/", "%2F"))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication if token is provided
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := rs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var versions []struct {
		Metadata struct {
			Container struct {
				Tags []string `json:"tags"`
			} `json:"container"`
		} `json:"metadata"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract all unique tags
	tagMap := make(map[string]bool)
	for _, version := range versions {
		for _, tag := range version.Metadata.Container.Tags {
			tagMap[tag] = true
		}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}

	// Sort tags
	sort.Strings(tags)

	rs.logger.Info("Fetched tags from GitHub Container Registry",
		"owner", owner,
		"package", packageName,
		"count", len(tags))

	return tags, nil
}

// FetchDockerHubTags fetches available tags from Docker Hub
func (rs *RegistryService) FetchDockerHubTags(image string) ([]string, error) {
	// Parse image name: library/nginx or myorg/myimage
	parts := strings.Split(image, "/")

	var namespace, repo string
	if len(parts) == 1 {
		namespace = "library"
		repo = parts[0]
	} else if len(parts) == 2 {
		namespace = parts[0]
		repo = parts[1]
	} else {
		return nil, fmt.Errorf("invalid Docker Hub image format: %s", image)
	}

	rs.logger.Info("Fetching tags from Docker Hub",
		"namespace", namespace,
		"repo", repo)

	// Docker Hub API v2
	url := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags?page_size=100",
		namespace, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := rs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode)
	}

	var response struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tags := make([]string, 0, len(response.Results))
	for _, result := range response.Results {
		tags = append(tags, result.Name)
	}

	rs.logger.Info("Fetched tags from Docker Hub",
		"namespace", namespace,
		"repo", repo,
		"count", len(tags))

	return tags, nil
}

// FetchTags fetches available tags from any registry
func (rs *RegistryService) FetchTags(image, token string) ([]string, error) {
	// Determine registry type based on image prefix
	if strings.HasPrefix(image, "ghcr.io/") {
		return rs.FetchGitHubContainerTags(image, token)
	} else if strings.Contains(image, "/") || !strings.Contains(image, ".") {
		// Docker Hub format (org/image or library image)
		return rs.FetchDockerHubTags(image)
	}

	return nil, fmt.Errorf("unsupported registry for image: %s", image)
}

// FilterTagsByStrategy filters tags based on version strategy
func (rs *RegistryService) FilterTagsByStrategy(tags []string, strategy *VersionStrategy) []string {
	if strategy == nil {
		return tags
	}

	filtered := make([]string, 0)

	for _, tag := range tags {
		if rs.matchesStrategy(tag, strategy) {
			filtered = append(filtered, tag)
		}
	}

	rs.logger.Debug("Filtered tags by strategy",
		"original_count", len(tags),
		"filtered_count", len(filtered),
		"strategy", strategy.Name)

	return filtered
}

// matchesStrategy checks if a tag matches all rules in the strategy
func (rs *RegistryService) matchesStrategy(tag string, strategy *VersionStrategy) bool {
	// Check each rule type
	for _, rule := range strategy.Rules {
		switch rule.Type {
		case "stability":
			if !rs.matchesStabilityRule(tag, &rule) {
				return false
			}
		case "pattern":
			if !rs.matchesPatternRule(tag, &rule) {
				return false
			}
		case "fixed-versions":
			if !rs.matchesFixedVersionsRule(tag, &rule) {
				return false
			}
			// semver and schedule rules are handled elsewhere
			// semver is used for version comparison logic
			// schedule is used for timing checks
		}
	}

	return true
}

// matchesStabilityRule checks if a tag matches stability requirements
func (rs *RegistryService) matchesStabilityRule(tag string, rule *StrategyRule) bool {
	isPrerelease := rs.isPrerelease(tag)
	isRC := rs.isReleaseCandidate(tag)

	// If it's a prerelease and not allowed
	if isPrerelease && rule.AllowPrerelease != nil && !*rule.AllowPrerelease {
		return false
	}

	// If it's an RC and not allowed
	if isRC && rule.AllowRCs != nil && !*rule.AllowRCs {
		return false
	}

	return true
}

// matchesPatternRule checks if a tag matches the required pattern
func (rs *RegistryService) matchesPatternRule(tag string, rule *StrategyRule) bool {
	if rule.Pattern == "" {
		return true
	}

	matched, err := regexp.MatchString(rule.Pattern, tag)
	if err != nil {
		rs.logger.Warn("Invalid pattern in strategy rule", "pattern", rule.Pattern, "error", err)
		return false
	}

	// If pattern is required and doesn't match, reject
	if rule.Required != nil && *rule.Required && !matched {
		return false
	}

	return matched
}

// matchesFixedVersionsRule checks if a tag is in the allowed versions list
func (rs *RegistryService) matchesFixedVersionsRule(tag string, rule *StrategyRule) bool {
	if len(rule.AllowedVersions) == 0 {
		return true
	}

	for _, allowed := range rule.AllowedVersions {
		if tag == allowed {
			return true
		}
	}

	return false
}

// isReleaseCandidate checks if a tag is a release candidate
func (rs *RegistryService) isReleaseCandidate(tag string) bool {
	lowerTag := strings.ToLower(tag)
	rcKeywords := []string{"rc", "release-candidate"}

	for _, keyword := range rcKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}

	return false
}

// isExcluded checks if a tag is in the exclude list (kept for backward compatibility)
func (rs *RegistryService) isExcluded(tag string, excludes []string) bool {
	for _, exclude := range excludes {
		if tag == exclude {
			return true
		}
	}
	return false
}

// isPinned checks if a tag is in the pinned list (kept for backward compatibility)
func (rs *RegistryService) isPinned(tag string, pinned []string) bool {
	for _, pin := range pinned {
		if tag == pin {
			return true
		}
	}
	return false
}

// isPrerelease checks if a tag appears to be a prerelease version
func (rs *RegistryService) isPrerelease(tag string) bool {
	lowerTag := strings.ToLower(tag)
	prereleaseKeywords := []string{"alpha", "beta", "rc", "dev", "snapshot", "nightly", "pre"}

	for _, keyword := range prereleaseKeywords {
		if strings.Contains(lowerTag, keyword) {
			return true
		}
	}

	return false
}

// GetLatestTag returns the most recent tag from a list
func (rs *RegistryService) GetLatestTag(tags []string) string {
	if len(tags) == 0 {
		return ""
	}

	// Simple approach: return the last tag after sorting
	// In production, you might want semantic version comparison
	sorted := make([]string, len(tags))
	copy(sorted, tags)
	sort.Strings(sorted)

	return sorted[len(sorted)-1]
}

// Credential management methods

// StoreCredential stores registry credentials
func (rs *RegistryService) StoreCredential(registryType, username, token string) error {
	if rs.credentialStore == nil {
		return fmt.Errorf("credential store not initialized")
	}

	rs.logger.Info("Storing registry credential",
		"registry", registryType,
		"username", username)

	return rs.credentialStore.StoreCredential(registryType, username, token)
}

// GetCredential retrieves registry credentials
func (rs *RegistryService) GetCredential(registryType string) (*store.RegistryCredential, error) {
	if rs.credentialStore == nil {
		return nil, fmt.Errorf("credential store not initialized")
	}

	return rs.credentialStore.GetCredential(registryType)
}

// ListCredentials lists all stored registry credentials
func (rs *RegistryService) ListCredentials() ([]store.RegistryCredential, error) {
	if rs.credentialStore == nil {
		return nil, fmt.Errorf("credential store not initialized")
	}

	return rs.credentialStore.ListCredentials()
}

// DeleteCredential deletes registry credentials
func (rs *RegistryService) DeleteCredential(registryType string) error {
	if rs.credentialStore == nil {
		return fmt.Errorf("credential store not initialized")
	}

	rs.logger.Info("Deleting registry credential",
		"registry", registryType)

	return rs.credentialStore.DeleteCredential(registryType)
}
