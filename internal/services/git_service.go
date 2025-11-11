package services

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MinaroShikuchi/lixy/internal/store"
)

// GitRepository represents a monitored Git repository
type GitRepository struct {
	URL           string
	Branch        string
	LocalPath     string
	LastCommitSHA string
	Token         string // GitHub token for authentication
}

// GitService manages Git repository operations for GitOps
type GitService struct {
	logger      *slog.Logger
	configStore *store.ConfigStore
	// Keep in-memory cache for fast access
	repositories map[string]*GitRepository
}

// NewGitService creates a new Git service
func NewGitService(logger *slog.Logger, configStore *store.ConfigStore) *GitService {
	gs := &GitService{
		logger:       logger,
		configStore:  configStore,
		repositories: make(map[string]*GitRepository),
	}

	// Load persisted repositories into cache
	gs.loadRepositoriesFromStore()

	return gs
}

// loadRepositoriesFromStore loads all repositories from persistent storage into memory
func (gs *GitService) loadRepositoriesFromStore() error {
	repoNames := gs.configStore.ListGitRepositories()

	for _, name := range repoNames {
		repoConfig, found := gs.configStore.GetGitRepositoryConfig(name)
		if !found {
			gs.logger.Warn("Repository config not found", "name", name)
			continue
		}

		gs.repositories[name] = &GitRepository{
			URL:       repoConfig.URL,
			Branch:    repoConfig.Branch,
			LocalPath: repoConfig.LocalPath,
			Token:     repoConfig.Token,
		}
		gs.logger.Info("Loaded repository from store",
			"name", name,
			"url", repoConfig.URL,
			"branch", repoConfig.Branch)
	}

	gs.logger.Info("Loaded repositories from persistent storage", "count", len(repoNames))
	return nil
}

// AddRepository adds a new repository to monitor and persists it
func (gs *GitService) AddRepository(name, url, branch, token string) error {
	// Create a local path for the repository
	localPath := filepath.Join("./data/repos", name)

	// Create repository struct
	repo := &GitRepository{
		URL:       url,
		Branch:    branch,
		LocalPath: localPath,
		Token:     token,
	}

	// Store in memory
	gs.repositories[name] = repo

	// Persist to storage using ConfigStore
	if err := gs.configStore.SetGitRepositoryConfig(name, url, branch, localPath, token); err != nil {
		// Remove from memory if storage fails
		delete(gs.repositories, name)
		return fmt.Errorf("failed to persist repository: %w", err)
	}

	gs.logger.Info("Added repository to monitor",
		"name", name,
		"url", url,
		"branch", branch)

	return nil
}

// CloneOrPull clones the repository if it doesn't exist, or pulls latest changes
func (gs *GitService) CloneOrPull(repoName string) (bool, error) {
	repo, exists := gs.repositories[repoName]
	if !exists {
		return false, fmt.Errorf("repository %s not found", repoName)
	}

	// Check if repository exists locally
	if _, err := os.Stat(repo.LocalPath); os.IsNotExist(err) {
		gs.logger.Info("Cloning repository", "name", repoName, "url", repo.URL)
		return true, gs.cloneRepository(repo)
	}

	// Repository exists, pull latest changes
	gs.logger.Debug("Pulling latest changes", "name", repoName)
	return gs.pullRepository(repo)
}

// cloneRepository clones a Git repository
func (gs *GitService) cloneRepository(repo *GitRepository) error {
	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(repo.LocalPath), 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Build clone URL with token if provided
	cloneURL := repo.URL
	if repo.Token != "" {
		// For GitHub URLs, inject token
		cloneURL = gs.injectTokenIntoURL(repo.URL, repo.Token)
	}

	// Clone the repository
	cmd := exec.Command("git", "clone", "-b", repo.Branch, cloneURL, repo.LocalPath)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0") // Disable password prompts

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to clone repository: %w\nOutput: %s", err, string(output))
	}

	// Get initial commit SHA
	commitSHA, err := gs.getCurrentCommitSHA(repo.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to get commit SHA: %w", err)
	}
	repo.LastCommitSHA = commitSHA

	gs.logger.Info("Successfully cloned repository",
		"path", repo.LocalPath,
		"commit", commitSHA)

	return nil
}

// pullRepository pulls latest changes from the repository
func (gs *GitService) pullRepository(repo *GitRepository) (bool, error) {
	// Get current commit SHA before pull
	oldCommitSHA, err := gs.getCurrentCommitSHA(repo.LocalPath)
	if err != nil {
		return false, fmt.Errorf("failed to get current commit SHA: %w", err)
	}

	// Configure Git to use token for authentication if provided
	if repo.Token != "" {
		credentialHelper := fmt.Sprintf("!f() { echo \"username=git\"; echo \"password=%s\"; }; f", repo.Token)
		cmd := exec.Command("git", "-C", repo.LocalPath, "config", "credential.helper", credentialHelper)
		if err := cmd.Run(); err != nil {
			gs.logger.Warn("Failed to configure credential helper", "error", err)
		}
	}

	// Fetch latest changes
	fetchCmd := exec.Command("git", "-C", repo.LocalPath, "fetch", "origin", repo.Branch)
	fetchCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if output, err := fetchCmd.CombinedOutput(); err != nil {
		return false, fmt.Errorf("failed to fetch: %w\nOutput: %s", err, string(output))
	}

	// Pull changes
	pullCmd := exec.Command("git", "-C", repo.LocalPath, "pull", "origin", repo.Branch)
	pullCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	output, err := pullCmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to pull: %w\nOutput: %s", err, string(output))
	}

	// Get new commit SHA
	newCommitSHA, err := gs.getCurrentCommitSHA(repo.LocalPath)
	if err != nil {
		return false, fmt.Errorf("failed to get new commit SHA: %w", err)
	}

	// Check if there were changes
	hasChanges := oldCommitSHA != newCommitSHA
	if hasChanges {
		gs.logger.Info("Repository updated with new changes",
			"old_commit", oldCommitSHA,
			"new_commit", newCommitSHA)
		repo.LastCommitSHA = newCommitSHA
	} else {
		gs.logger.Debug("No new changes in repository", "commit", newCommitSHA)
	}

	return hasChanges, nil
}

// getCurrentCommitSHA gets the current commit SHA of the repository
func (gs *GitService) getCurrentCommitSHA(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRepositoryPath returns the local path of a repository
func (gs *GitService) GetRepositoryPath(repoName string) (string, error) {
	repo, exists := gs.repositories[repoName]
	if !exists {
		return "", fmt.Errorf("repository %s not found", repoName)
	}
	return repo.LocalPath, nil
}

// injectTokenIntoURL injects a GitHub token into the repository URL
func (gs *GitService) injectTokenIntoURL(url, token string) string {
	// For HTTPS URLs like https://github.com/owner/repo.git
	if strings.HasPrefix(url, "https://github.com/") {
		return strings.Replace(url, "https://", fmt.Sprintf("https://%s@", token), 1)
	}
	// For other HTTPS URLs
	if strings.HasPrefix(url, "https://") {
		return strings.Replace(url, "https://", fmt.Sprintf("https://oauth2:%s@", token), 1)
	}
	return url
}

// DeleteRepository removes a repository from monitoring and optionally deletes local files
func (gs *GitService) DeleteRepository(name string, deleteLocal bool) error {
	repo, exists := gs.repositories[name]
	if !exists {
		return fmt.Errorf("repository %s not found", name)
	}

	// Delete local repository if requested
	if deleteLocal && repo.LocalPath != "" {
		if err := os.RemoveAll(repo.LocalPath); err != nil {
			gs.logger.Error("Failed to delete local repository", "name", name, "path", repo.LocalPath, "error", err)
			return fmt.Errorf("failed to delete local repository: %w", err)
		}
		gs.logger.Info("Deleted local repository files", "name", name, "path", repo.LocalPath)
	}

	// Remove from repositories map
	delete(gs.repositories, name)
	gs.logger.Info("Removed repository from monitoring", "name", name)

	return nil
}

// ListRepositories returns all monitored repositories
func (gs *GitService) ListRepositories() []string {
	repos := make([]string, 0, len(gs.repositories))
	for name := range gs.repositories {
		repos = append(repos, name)
	}
	return repos
}

// GetRepository returns a repository by name
func (gs *GitService) GetRepository(name string) (*GitRepository, error) {
	repo, exists := gs.repositories[name]
	if !exists {
		return nil, fmt.Errorf("repository %s not found", name)
	}
	return repo, nil
}

// ListEnvironments returns all environments available in a repository
func (gs *GitService) ListEnvironments(repoName string) ([]string, error) {
	repo, exists := gs.repositories[repoName]
	if !exists {
		return nil, fmt.Errorf("repository %s not found", repoName)
	}

	environmentsPath := filepath.Join(repo.LocalPath, "environments")

	// Check if environments directory exists
	if _, err := os.Stat(environmentsPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	entries, err := os.ReadDir(environmentsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read environments directory: %w", err)
	}

	environments := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			environments = append(environments, entry.Name())
		}
	}

	gs.logger.Info("Listed environments in repository",
		"repository", repoName,
		"count", len(environments))

	return environments, nil
}

// MonitorRepository starts monitoring a repository for changes
func (gs *GitService) MonitorRepository(repoName string, interval time.Duration, onChange func()) {
	gs.logger.Info("Starting repository monitor",
		"name", repoName,
		"interval", interval)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		hasChanges, err := gs.CloneOrPull(repoName)
		if err != nil {
			gs.logger.Error("Failed to sync repository",
				"name", repoName,
				"error", err)
			continue
		}

		if hasChanges && onChange != nil {
			gs.logger.Info("Repository changed, triggering callback", "name", repoName)
			onChange()
		}
	}
}

// RemoveRepository removes a repository from monitoring and deletes persisted data
func (gs *GitService) RemoveRepository(name string) error {
	// Remove from memory
	delete(gs.repositories, name)

	// Remove from persistent storage
	if err := gs.configStore.DeleteGitRepositoryConfig(name); err != nil {
		return fmt.Errorf("failed to remove repository from storage: %w", err)
	}

	gs.logger.Info("Removed repository from monitoring", "name", name)
	return nil
}

// UpdateRepositoryToken updates the token for an existing repository
func (gs *GitService) UpdateRepositoryToken(name, token string) error {
	repo, exists := gs.repositories[name]
	if !exists {
		return fmt.Errorf("repository %s not found", name)
	}

	// Update in memory
	repo.Token = token

	// Update in storage
	if err := gs.configStore.UpdateGitRepositoryToken(name, token); err != nil {
		return fmt.Errorf("failed to update token in storage: %w", err)
	}

	gs.logger.Info("Updated repository token", "name", name)
	return nil
}

// GetRepositoryDetails returns all monitored repositories with their details
func (gs *GitService) GetRepositoryDetails() map[string]*GitRepository {
	// Return a copy to prevent external modifications
	result := make(map[string]*GitRepository)
	for name, repo := range gs.repositories {
		result[name] = &GitRepository{
			URL:           repo.URL,
			Branch:        repo.Branch,
			LocalPath:     repo.LocalPath,
			LastCommitSHA: repo.LastCommitSHA,
			Token:         repo.Token,
		}
	}
	return result
}
