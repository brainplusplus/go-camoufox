package pkgman

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type InstalledVersion struct {
	Path      string
	Version   Version
	IsActive  bool
	Repo      string
	Channel   string
	LaunchExe string
}

type VersionMetadata struct {
	Version        string `json:"version,omitempty"`
	Build          string `json:"build"`
	Repo           string `json:"repo,omitempty"`
	RepoName       string `json:"repo_name,omitempty"`
	Prerelease     bool   `json:"prerelease,omitempty"`
	AssetID        int64  `json:"asset_id,omitempty"`
	AssetSize      int64  `json:"asset_size,omitempty"`
	AssetUpdatedAt string `json:"asset_updated_at,omitempty"`
}

func CacheDir() (string, error) {
	if value := os.Getenv("GO_CAMOUFOX_CACHE"); value != "" {
		return value, nil
	}
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "go-camoufox"), nil
}

func BrowsersDir() (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return browsersDir(dir), nil
}

func browsersDir(cacheDir string) string {
	return filepath.Join(cacheDir, "browsers")
}

func LaunchFile(goos string) (string, error) {
	switch goos {
	case "windows":
		return "camoufox.exe", nil
	case "darwin":
		return filepath.Join("..", "MacOS", "camoufox"), nil
	case "linux":
		return "camoufox-bin", nil
	default:
		return "", fmt.Errorf("os %s is not supported", goos)
	}
}

func LaunchPath(browserPath string) (string, error) {
	if browserPath == "" {
		if config, err := LoadConfig(); err == nil && config.ActiveVersion != "" {
			if item, findErr := FindInstalled(config.ActiveVersion); findErr == nil {
				return item.LaunchExe, nil
			}
		}
		installed, err := ListInstalled()
		if err != nil {
			return "", err
		}
		for _, item := range installed {
			if item.IsActive {
				return item.LaunchExe, nil
			}
		}
		if len(installed) > 0 {
			return installed[0].LaunchExe, nil
		}
		return "", fmt.Errorf("camoufox is not installed; run go-camoufox fetch")
	}
	launchFile, err := LaunchFile(runtime.GOOS)
	if err != nil {
		return "", err
	}
	return filepath.Join(browserPath, launchFile), nil
}

func VerifyLaunchPath(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("camoufox executable %q is not accessible: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("camoufox executable %q is a directory", path)
	}
	return nil
}

func FindInstalledLaunchPath(specifier string) (string, error) {
	installed, err := ListInstalled()
	if err != nil {
		return "", err
	}
	specifier = strings.ToLower(strings.TrimSpace(specifier))
	for _, item := range installed {
		candidates := []string{
			strings.ToLower(item.Version.Build),
			strings.ToLower(item.Version.FullString()),
			strings.ToLower(item.Channel),
			strings.ToLower(filepath.Base(item.Path)),
		}
		if item.Repo != "" && item.Channel != "" {
			candidates = append(candidates, strings.ToLower(item.Repo+"/"+item.Channel))
		}
		for _, candidate := range candidates {
			if candidate == specifier {
				return item.LaunchExe, nil
			}
		}
	}
	return "", fmt.Errorf("browser version %q not found", specifier)
}

func ListInstalled() ([]InstalledVersion, error) {
	root, err := BrowsersDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]InstalledVersion, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name())
		if item, ok := readInstalled(path); ok {
			out = append(out, item)
			continue
		}
		children, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, child := range children {
			if !child.IsDir() {
				continue
			}
			if item, ok := readInstalled(filepath.Join(path, child.Name())); ok {
				if item.Repo == "" {
					item.Repo = entry.Name()
				}
				out = append(out, item)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Repo != out[j].Repo {
			return out[i].Repo < out[j].Repo
		}
		return out[i].Version.Compare(out[j].Version) > 0
	})
	return out, nil
}

func readInstalled(path string) (InstalledVersion, bool) {
	versionPath := filepath.Join(path, "version.json")
	metadata, err := ReadVersionMetadata(versionPath)
	if err != nil {
		return InstalledVersion{}, false
	}
	launch, err := LaunchPath(path)
	if err != nil {
		return InstalledVersion{}, false
	}
	if err := VerifyLaunchPath(launch); err != nil {
		return InstalledVersion{}, false
	}
	return InstalledVersion{
		Path:      path,
		Version:   Version{Build: metadata.Build, Version: metadata.Version},
		Repo:      metadata.RepoName,
		Channel:   metadata.Build,
		LaunchExe: launch,
		IsActive:  isActiveInstalled(path, metadata),
	}, true
}

func isActiveInstalled(path string, metadata VersionMetadata) bool {
	config, err := LoadConfig()
	if err != nil {
		return false
	}
	item := InstalledVersion{
		Path:    path,
		Version: Version{Build: metadata.Build, Version: metadata.Version},
		Repo:    metadata.RepoName,
	}
	return config.ActiveVersion != "" && config.ActiveVersion == RelativeInstalledPath(item)
}

func ReadVersionMetadata(path string) (VersionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return VersionMetadata{}, err
	}
	var metadata VersionMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return VersionMetadata{}, err
	}
	if metadata.Build == "" {
		var legacy struct {
			Build   string `json:"build"`
			Release string `json:"release"`
			Tag     string `json:"tag"`
			Version string `json:"version"`
		}
		if err := json.Unmarshal(data, &legacy); err != nil {
			return VersionMetadata{}, err
		}
		metadata.Build = legacy.Build
		if metadata.Build == "" {
			metadata.Build = legacy.Release
		}
		if metadata.Build == "" {
			metadata.Build = legacy.Tag
		}
		metadata.Version = legacy.Version
	}
	if metadata.Build == "" {
		return VersionMetadata{}, fmt.Errorf("version metadata %q does not contain build, release, or tag", path)
	}
	return metadata, nil
}

func WriteVersionMetadata(dir string, selected AvailableVersion) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	metadata := VersionMetadata{
		Version:    selected.Version.Version,
		Build:      selected.Version.Build,
		Repo:       selected.Repo,
		RepoName:   selected.RepoName,
		Prerelease: selected.IsPrerelease,
		AssetID:    selected.AssetID,
		AssetSize:  selected.AssetSize,
	}
	if !selected.AssetUpdatedAt.IsZero() {
		metadata.AssetUpdatedAt = selected.AssetUpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "version.json"), data, 0o644)
}
