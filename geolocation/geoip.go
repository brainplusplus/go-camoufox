package geolocation

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/brainplusplus/go-camoufox/internal/assets"
	"github.com/oschwald/maxminddb-golang"
	"gopkg.in/yaml.v3"
)

const geoIPUpdateDays = 30

type GeoIPRepo struct {
	Name    string             `yaml:"name"`
	Extract bool               `yaml:"extract"`
	URLs    map[string]URLList `yaml:"urls"`
	Paths   GeoIPPaths         `yaml:"paths"`
}

type URLList []string

func (u *URLList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		*u = []string{value.Value}
		return nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(value.Content))
		for _, item := range value.Content {
			out = append(out, item.Value)
		}
		*u = out
		return nil
	default:
		return fmt.Errorf("invalid URL list YAML node kind %d", value.Kind)
	}
}

type GeoIPPaths struct {
	ISOCode   string `yaml:"iso_code"`
	Longitude string `yaml:"longitude"`
	Latitude  string `yaml:"latitude"`
	Timezone  string `yaml:"timezone"`
}

type geoIPReposYAML struct {
	Default struct {
		GeoIP string `yaml:"geoip"`
	} `yaml:"default"`
	GeoIP []GeoIPRepo `yaml:"geoip"`
}

var GeoIPHTTPClient HTTPDoer = &http.Client{Timeout: 60 * time.Second}

func LoadGeoIPRepos() ([]GeoIPRepo, string, error) {
	data, err := assets.ReadFile("repos.yml")
	if err != nil {
		return nil, "", err
	}
	var parsed geoIPReposYAML
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return nil, "", err
	}
	return parsed.GeoIP, parsed.Default.GeoIP, nil
}

func GetGeoIPConfigByName(name string) (GeoIPRepo, error) {
	repos, defaultName, err := LoadGeoIPRepos()
	if err != nil {
		return GeoIPRepo{}, err
	}
	target := name
	if target == "" {
		target = defaultName
	}
	for _, repo := range repos {
		if strings.EqualFold(repo.Name, target) {
			if len(repo.URLs) == 0 {
				return GeoIPRepo{}, fmt.Errorf("GeoIP repo %q missing required urls", repo.Name)
			}
			if repo.Paths.ISOCode == "" || repo.Paths.Longitude == "" || repo.Paths.Latitude == "" || repo.Paths.Timezone == "" {
				return GeoIPRepo{}, fmt.Errorf("GeoIP repo %q missing required paths", repo.Name)
			}
			return repo, nil
		}
	}
	if name != "" {
		names := make([]string, 0, len(repos))
		for _, repo := range repos {
			names = append(names, repo.Name)
		}
		return GeoIPRepo{}, fmt.Errorf("GeoIP database %q not found. Available: %v", name, names)
	}
	if len(repos) == 0 {
		return GeoIPRepo{}, fmt.Errorf("no GeoIP repos configured in repos.yml")
	}
	return repos[0], nil
}

func GeoIPDir() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "camoufox", "geoip"), nil
}

func MMDBDir() (string, error) {
	root, err := GeoIPDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "mmdb"), nil
}

func MMDBPath(ipVersion string, repo GeoIPRepo) (string, error) {
	dir, err := MMDBDir()
	if err != nil {
		return "", err
	}
	name := strings.ToLower(strings.ReplaceAll(repo.Name, " ", "-"))
	if _, ok := repo.URLs["combined"]; ok {
		return filepath.Join(dir, name+"-combined.mmdb"), nil
	}
	return filepath.Join(dir, name+"-"+ipVersion+".mmdb"), nil
}

func NeedsUpdate(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > geoIPUpdateDays*24*time.Hour
}

func DownloadMMDB(ctx context.Context, source string) error {
	repo, err := GetGeoIPConfigByName(source)
	if err != nil {
		return err
	}
	dir, err := MMDBDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for ipVersion, urls := range repo.URLs {
		mmdbPath, err := MMDBPath(ipVersion, repo)
		if err != nil {
			return err
		}
		var lastErr error
		for _, endpoint := range urls {
			lastErr = downloadOne(ctx, endpoint, repo.Extract, mmdbPath)
			if lastErr == nil {
				break
			}
		}
		if lastErr != nil {
			return lastErr
		}
	}
	return nil
}

func RemoveMMDB() error {
	dir, err := GeoIPDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(dir)
}

func GetGeolocation(ctx context.Context, ip string, geoipDB string) (*Geolocation, error) {
	if err := ValidatePublicIP(ip); err != nil {
		return nil, err
	}
	repo, err := GetGeoIPConfigByName(geoipDB)
	if err != nil {
		return nil, err
	}
	ipVersion := "ipv4"
	if ValidIPv6(ip) {
		ipVersion = "ipv6"
	}
	mmdbPath, err := MMDBPath(ipVersion, repo)
	if err != nil {
		return nil, err
	}
	if NeedsUpdate(mmdbPath) {
		if err := DownloadMMDB(ctx, geoipDB); err != nil {
			return nil, err
		}
	}
	reader, err := maxminddb.Open(mmdbPath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var record map[string]any
	if err := reader.Lookup(netIP(ip), &record); err != nil {
		return nil, err
	}
	if record == nil {
		return nil, fmt.Errorf("IP not found in database: %s", ip)
	}
	isoCode := strings.ToUpper(fmt.Sprint(findIn(record, repo.Paths.ISOCode)))
	longitude, err := floatFromAny(findIn(record, repo.Paths.Longitude))
	if err != nil {
		return nil, fmt.Errorf("invalid longitude for %s: %w", ip, err)
	}
	latitude, err := floatFromAny(findIn(record, repo.Paths.Latitude))
	if err != nil {
		return nil, fmt.Errorf("invalid latitude for %s: %w", ip, err)
	}
	timezone := fmt.Sprint(findIn(record, repo.Paths.Timezone))
	selector, err := defaultSelector()
	if err != nil {
		return nil, err
	}
	locale, err := selector.FromRegion(isoCode)
	if err != nil {
		return nil, err
	}
	return &Geolocation{Locale: locale, Longitude: longitude, Latitude: latitude, Timezone: timezone}, nil
}

func downloadOne(ctx context.Context, endpoint string, extract bool, target string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := GeoIPHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s returned %s", endpoint, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), "geoip-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if extract {
		return extractFirstMMDB(tmpName, target)
	}
	return os.Rename(tmpName, target)
}

func extractFirstMMDB(zipPath, target string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()
	for _, file := range reader.File {
		if !strings.HasSuffix(strings.ToLower(file.Name), ".mmdb") {
			continue
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		defer src.Close()
		dst, err := os.Create(target)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(dst, src)
		closeErr := dst.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return fmt.Errorf("no .mmdb file found in archive")
}

func findIn(data map[string]any, path string) any {
	var current any = data
	for _, part := range strings.Split(path, ".") {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[part]
		if current == nil {
			return nil
		}
	}
	return current
}

func floatFromAny(value any) (float64, error) {
	switch typed := value.(type) {
	case float64:
		return typed, nil
	case float32:
		return float64(typed), nil
	case int:
		return float64(typed), nil
	case uint:
		return float64(typed), nil
	case string:
		var out float64
		if _, err := fmt.Sscanf(typed, "%f", &out); err != nil {
			return 0, err
		}
		return out, nil
	default:
		return 0, fmt.Errorf("unsupported value %T", value)
	}
}

func netIP(value string) net.IP {
	return net.ParseIP(value)
}
