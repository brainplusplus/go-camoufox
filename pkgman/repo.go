package pkgman

import (
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/brainplusplus/go-camoufox/internal/assets"
	"gopkg.in/yaml.v3"
)

type RepoConfig struct {
	Repos    []string
	Name     string
	Pattern  string
	BuildMin string
	BuildMax string
}

type reposYAML struct {
	Default struct {
		Browser string `yaml:"browser"`
		GeoIP   string `yaml:"geoip"`
	} `yaml:"default"`
	Browsers []browserRepoYAML `yaml:"browsers"`
}

type browserRepoYAML struct {
	Repo     string              `yaml:"repo"`
	Name     string              `yaml:"name"`
	Pattern  string              `yaml:"pattern"`
	Versions []versionConstraint `yaml:"versions"`
}

type versionConstraint struct {
	PythonLibrary rangeYAML `yaml:"python_library"`
	Browser       rangeYAML `yaml:"browser"`
}

type rangeYAML struct {
	Min string `yaml:"min"`
	Max string `yaml:"max"`
}

func LoadRepos(spoofLibraryVersion string) ([]RepoConfig, error) {
	data, err := assets.ReadFile("repos.yml")
	if err != nil {
		return nil, err
	}
	var parsed reposYAML
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	repos := make([]RepoConfig, 0, len(parsed.Browsers))
	for _, item := range parsed.Browsers {
		repo := RepoConfig{
			Repos:   splitRepos(item.Repo),
			Name:    item.Name,
			Pattern: item.Pattern,
		}
		if len(item.Versions) > 0 {
			constraint := findVersionConstraint(item.Versions, spoofLibraryVersion)
			repo.BuildMin = constraint.Min
			repo.BuildMax = constraint.Max
		}
		repos = append(repos, repo)
	}
	return repos, nil
}

func GetDefaultRepo() (RepoConfig, error) {
	data, err := assets.ReadFile("repos.yml")
	if err != nil {
		return RepoConfig{}, err
	}
	var parsed reposYAML
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return RepoConfig{}, err
	}
	repos, err := LoadRepos("")
	if err != nil {
		return RepoConfig{}, err
	}
	for _, repo := range repos {
		if strings.EqualFold(repo.Name, parsed.Default.Browser) {
			return repo, nil
		}
	}
	if len(repos) == 0 {
		return RepoConfig{}, fmt.Errorf("no browser repositories configured")
	}
	return repos[0], nil
}

func (r RepoConfig) BuildPattern(goos, goarch string) (*regexp.Regexp, error) {
	osName, err := MapOS(goos)
	if err != nil {
		return nil, err
	}
	arch, err := MapArch(goarch)
	if err != nil {
		return nil, err
	}
	replacements := map[string]string{
		"name":    `(?P<name>\w+)`,
		"version": `(?P<version>[^-]+)`,
		"build":   `(?P<build>[^-]+)`,
		"os":      regexp.QuoteMeta(osName),
		"arch":    regexp.QuoteMeta(arch),
	}
	pattern := strings.ReplaceAll(r.Pattern, ".", `\.`)
	re := regexp.MustCompile(`\{(\w+)\}`)
	regex := re.ReplaceAllStringFunc(pattern, func(match string) string {
		key := strings.Trim(match, "{}")
		if value, ok := replacements[key]; ok {
			return value
		}
		return match
	})
	return regexp.Compile("^" + regex + "$")
}

func (r RepoConfig) IsVersionSupported(version Version) bool {
	if r.BuildMin == "" || r.BuildMax == "" {
		return true
	}
	return version.Compare(Version{Build: r.BuildMin}) >= 0 && version.Compare(Version{Build: r.BuildMax}) <= 0
}

func MapOS(goos string) (string, error) {
	switch goos {
	case "darwin":
		return "mac", nil
	case "linux":
		return "lin", nil
	case "windows":
		return "win", nil
	default:
		return "", fmt.Errorf("os %s is not supported", goos)
	}
}

func MapArch(goarch string) (string, error) {
	switch strings.ToLower(goarch) {
	case "amd64", "x86_64", "x86":
		return "x86_64", nil
	case "386", "i386", "i686":
		return "i686", nil
	case "arm64", "aarch64", "armv5l", "armv6l", "armv7l":
		return "arm64", nil
	default:
		return "", fmt.Errorf("architecture %s is not supported", goarch)
	}
}

func CurrentPlatformArch() (string, error) {
	osName, err := MapOS(runtime.GOOS)
	if err != nil {
		return "", err
	}
	arch, err := MapArch(runtime.GOARCH)
	if err != nil {
		return "", err
	}
	if !supportedOSArch(osName, arch) {
		return "", fmt.Errorf("architecture %s is not supported for %s", arch, osName)
	}
	return arch, nil
}

func splitRepos(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func findVersionConstraint(items []versionConstraint, libraryVersion string) rangeYAML {
	if libraryVersion == "" {
		libraryVersion = "0.5.0"
	}
	for _, item := range items {
		if semverInRange(libraryVersion, item.PythonLibrary.Min, item.PythonLibrary.Max) {
			return item.Browser
		}
	}
	return rangeYAML{}
}

func semverInRange(version, min, max string) bool {
	v := parseSemver(version)
	return compareIntSlices(v, parseSemver(min)) >= 0 && compareIntSlices(v, parseSemver(max)) < 0
}

func parseSemver(value string) []int {
	value = strings.TrimLeft(value, "^~")
	parts := strings.Split(value, ".")
	out := []int{0, 0, 0}
	for i := 0; i < len(parts) && i < len(out); i++ {
		if n, err := strconv.Atoi(parts[i]); err == nil {
			out[i] = n
		}
	}
	return out
}

func compareIntSlices(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		left, right := 0, 0
		if i < len(a) {
			left = a[i]
		}
		if i < len(b) {
			right = b[i]
		}
		if left < right {
			return -1
		}
		if left > right {
			return 1
		}
	}
	return 0
}

func supportedOSArch(osName, arch string) bool {
	matrix := map[string]map[string]struct{}{
		"win": {"x86_64": {}, "i686": {}},
		"mac": {"x86_64": {}, "arm64": {}},
		"lin": {"x86_64": {}, "arm64": {}, "i686": {}},
	}
	_, ok := matrix[osName][arch]
	return ok
}
