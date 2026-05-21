package pkgman

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	defaultChannel    = "official/stable"
	configFileName    = "config.json"
	repoCacheFileName = "repo-cache.json"
)

type Config struct {
	Channel       string `json:"channel,omitempty"`
	Pinned        string `json:"pinned,omitempty"`
	ActiveVersion string `json:"active_version,omitempty"`
}

type RepoCache struct {
	Repos []RepoCacheRepo `json:"repos"`
}

type RepoCacheRepo struct {
	Name     string             `json:"name"`
	Repos    []string           `json:"repos,omitempty"`
	Versions []RepoCacheVersion `json:"versions"`
}

type RepoCacheVersion struct {
	Version        string `json:"version"`
	Build          string `json:"build"`
	URL            string `json:"url"`
	IsPrerelease   bool   `json:"is_prerelease"`
	AssetID        int64  `json:"asset_id,omitempty"`
	AssetSize      int64  `json:"asset_size,omitempty"`
	AssetUpdatedAt string `json:"asset_updated_at,omitempty"`
}

func DefaultChannel() string {
	return defaultChannel
}

func ConfigPath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func RepoCachePath() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, repoCacheFileName), nil
}

func LoadConfig() (Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return Config{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Config{Channel: defaultChannel}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	if config.Channel == "" {
		config.Channel = defaultChannel
	}
	return config, nil
}

func SaveConfig(config Config) error {
	if config.Channel == "" {
		config.Channel = defaultChannel
	}
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func LoadRepoCache() (RepoCache, error) {
	path, err := RepoCachePath()
	if err != nil {
		return RepoCache{}, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return RepoCache{}, nil
	}
	if err != nil {
		return RepoCache{}, err
	}
	var cache RepoCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return RepoCache{}, err
	}
	return cache, nil
}

func SaveRepoCache(cache RepoCache) error {
	path, err := RepoCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func SyncRepoCache(ctx context.Context, opts InstallOptions) (RepoCache, error) {
	repos, err := LoadRepos("")
	if err != nil {
		return RepoCache{}, err
	}
	cache := RepoCache{Repos: make([]RepoCacheRepo, 0, len(repos))}
	for _, repo := range repos {
		versions, err := ListAvailableVersions(ctx, InstallOptions{
			Repo:              repo,
			Client:            opts.Client,
			IncludePrerelease: true,
		})
		if err != nil {
			continue
		}
		entry := RepoCacheRepo{Name: repo.Name, Repos: append([]string(nil), repo.Repos...)}
		for _, version := range versions {
			item := RepoCacheVersion{
				Version:      version.Version.Version,
				Build:        version.Version.Build,
				URL:          version.URL,
				IsPrerelease: version.IsPrerelease,
				AssetID:      version.AssetID,
				AssetSize:    version.AssetSize,
			}
			if !version.AssetUpdatedAt.IsZero() {
				item.AssetUpdatedAt = version.AssetUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			entry.Versions = append(entry.Versions, item)
		}
		cache.Repos = append(cache.Repos, entry)
	}
	if len(cache.Repos) == 0 {
		return RepoCache{}, fmt.Errorf("no repositories could be synced")
	}
	if err := SaveRepoCache(cache); err != nil {
		return RepoCache{}, err
	}
	return cache, nil
}

func SetChannel(specifier string) (Config, error) {
	repo, channel, ok := splitChannel(specifier)
	if !ok || channel != "stable" && channel != "prerelease" {
		return Config{}, fmt.Errorf("expected repo/stable or repo/prerelease, got %q", specifier)
	}
	config, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	config.Channel = strings.ToLower(repo + "/" + channel)
	config.Pinned = ""
	config.ActiveVersion = ""
	if installed, _ := LatestInstalledForChannel(repo, channel); installed != nil {
		config.ActiveVersion = RelativeInstalledPath(*installed)
	}
	return config, SaveConfig(config)
}

func SetPinned(specifier string) (Config, error) {
	repo, channel, version, ok := splitPinned(specifier)
	if !ok || channel != "stable" && channel != "prerelease" {
		return Config{}, fmt.Errorf("expected repo/channel/version, got %q", specifier)
	}
	config, err := LoadConfig()
	if err != nil {
		return Config{}, err
	}
	config.Channel = strings.ToLower(repo + "/" + channel)
	config.Pinned = strings.TrimPrefix(version, "v")
	config.ActiveVersion = ""
	if installed, _ := FindInstalled(specifier); installed != nil {
		config.ActiveVersion = RelativeInstalledPath(*installed)
	}
	return config, SaveConfig(config)
}

func ActiveDisplay() (string, bool, error) {
	config, err := LoadConfig()
	if err != nil {
		return "", false, err
	}
	if config.Pinned != "" {
		display := strings.ToLower(config.Channel) + "/" + config.Pinned
		if installed, _ := FindInstalled(display); installed != nil {
			return InstalledChannelPath(*installed), true, nil
		}
		return display, false, nil
	}
	if config.ActiveVersion != "" {
		if installed, _ := FindInstalled(config.ActiveVersion); installed != nil {
			return InstalledChannelPath(*installed), true, nil
		}
	}
	repo, channel, _ := splitChannel(config.Channel)
	if installed, _ := LatestInstalledForChannel(repo, channel); installed != nil {
		return InstalledChannelPath(*installed), true, nil
	}
	return strings.ToLower(config.Channel), false, nil
}

func FindInstalled(specifier string) (*InstalledVersion, error) {
	installed, err := ListInstalled()
	if err != nil {
		return nil, err
	}
	spec := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(specifier, "v")))
	for _, item := range installed {
		candidates := []string{
			strings.ToLower(RelativeInstalledPath(item)),
			strings.ToLower(InstalledChannelPath(item)),
			strings.ToLower(item.Version.Build),
			strings.ToLower(item.Version.FullString()),
			strings.ToLower(filepath.Base(item.Path)),
		}
		if item.Repo != "" {
			candidates = append(candidates, strings.ToLower(item.Repo+"/"+item.Version.FullString()))
		}
		for _, candidate := range candidates {
			if candidate == spec {
				itemCopy := item
				return &itemCopy, nil
			}
		}
	}
	repo, channel, ok := splitChannel(spec)
	if ok && (channel == "stable" || channel == "prerelease") {
		return LatestInstalledForChannel(repo, channel)
	}
	return nil, fmt.Errorf("browser version %q not found", specifier)
}

func LatestInstalledForChannel(repo, channel string) (*InstalledVersion, error) {
	installed, err := ListInstalled()
	if err != nil {
		return nil, err
	}
	wantPre := channel == "prerelease"
	var matches []InstalledVersion
	for _, item := range installed {
		if !strings.EqualFold(item.Repo, repo) {
			continue
		}
		if strings.Contains(item.Version.Build, "beta") != wantPre {
			continue
		}
		matches = append(matches, item)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no installed version for %s/%s", repo, channel)
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Version.Compare(matches[j].Version) > 0
	})
	return &matches[0], nil
}

func SetActive(specifier string) (*InstalledVersion, error) {
	item, err := FindInstalled(specifier)
	if err != nil {
		return nil, err
	}
	config, err := LoadConfig()
	if err != nil {
		return nil, err
	}
	config.ActiveVersion = RelativeInstalledPath(*item)
	config.Channel = channelForInstalled(*item)
	config.Pinned = item.Version.FullString()
	if err := SaveConfig(config); err != nil {
		return nil, err
	}
	return item, nil
}

func RemoveInstalled(specifier string) (*InstalledVersion, error) {
	item, err := FindInstalled(specifier)
	if err != nil {
		return nil, err
	}
	if err := os.RemoveAll(item.Path); err != nil {
		return nil, err
	}
	config, _ := LoadConfig()
	if config.ActiveVersion == RelativeInstalledPath(*item) {
		config.ActiveVersion = ""
		_ = SaveConfig(config)
	}
	return item, nil
}

func RemoveAll() error {
	root, err := BrowsersDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(root)
}

func RelativeInstalledPath(item InstalledVersion) string {
	if item.Repo != "" {
		return strings.ToLower(item.Repo) + "/" + item.Version.FullString()
	}
	return item.Version.FullString()
}

func InstalledChannelPath(item InstalledVersion) string {
	return channelForInstalled(item) + "/" + item.Version.FullString()
}

func channelForInstalled(item InstalledVersion) string {
	channel := "stable"
	if strings.Contains(item.Version.Build, "beta") {
		channel = "prerelease"
	}
	repo := strings.ToLower(item.Repo)
	if repo == "" {
		repo = "official"
	}
	return repo + "/" + channel
}

func splitChannel(specifier string) (string, string, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(specifier)), "/")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], parts[0] != "" && parts[1] != ""
}

func splitPinned(specifier string) (string, string, string, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(specifier)), "/")
	if len(parts) != 3 {
		return "", "", "", false
	}
	return parts[0], parts[1], strings.TrimPrefix(parts[2], "v"), parts[0] != "" && parts[1] != "" && parts[2] != ""
}
