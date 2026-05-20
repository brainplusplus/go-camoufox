package pkgman

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type ReleaseAsset struct {
	ID                 int64     `json:"id,omitempty"`
	Name               string    `json:"name"`
	BrowserDownloadURL string    `json:"browser_download_url"`
	Size               int64     `json:"size,omitempty"`
	UpdatedAt          time.Time `json:"updated_at,omitempty"`
}

type Release struct {
	Prerelease bool           `json:"prerelease"`
	Assets     []ReleaseAsset `json:"assets"`
}

type AvailableVersion struct {
	Version        Version
	URL            string
	IsPrerelease   bool
	Repo           string
	RepoName       string
	AssetID        int64
	AssetSize      int64
	AssetUpdatedAt time.Time
}

type CamoufoxFetcher struct {
	RepoConfig RepoConfig
	OS         string
	Arch       string
	Version    *Version
	Pattern    *regexp.Regexp
}

func NewFetcher(repo RepoConfig, goos, goarch string) (*CamoufoxFetcher, error) {
	pattern, err := repo.BuildPattern(goos, goarch)
	if err != nil {
		return nil, err
	}
	arch, err := MapArch(goarch)
	if err != nil {
		return nil, err
	}
	osName, err := MapOS(goos)
	if err != nil {
		return nil, err
	}
	return &CamoufoxFetcher{
		RepoConfig: repo,
		OS:         osName,
		Arch:       arch,
		Pattern:    pattern,
	}, nil
}

func (f *CamoufoxFetcher) CheckAsset(asset ReleaseAsset, release Release) (*AvailableVersion, error) {
	matches := f.Pattern.FindStringSubmatch(asset.Name)
	if matches == nil {
		return nil, nil
	}
	values := map[string]string{}
	for i, name := range f.Pattern.SubexpNames() {
		if i > 0 && name != "" {
			values[name] = matches[i]
		}
	}
	version := Version{Build: values["build"], Version: values["version"]}
	if !f.RepoConfig.IsVersionSupported(version) {
		return nil, nil
	}
	return &AvailableVersion{
		Version:        version,
		URL:            asset.BrowserDownloadURL,
		IsPrerelease:   release.Prerelease,
		RepoName:       f.RepoConfig.Name,
		AssetID:        asset.ID,
		AssetSize:      asset.Size,
		AssetUpdatedAt: asset.UpdatedAt,
	}, nil
}

func (f *CamoufoxFetcher) FetchLatestFrom(releases []Release) (*AvailableVersion, error) {
	for _, release := range releases {
		for _, asset := range release.Assets {
			version, err := f.CheckAsset(asset, release)
			if err != nil {
				return nil, err
			}
			if version != nil {
				return version, nil
			}
		}
	}
	return nil, fmt.Errorf("no matching release found for %s %s in supported range", f.OS, f.Arch)
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type InstallOptions struct {
	Repo              RepoConfig
	Client            HTTPDoer
	IncludePrerelease bool
	VersionSpec       string
	Replace           bool
	DownloadWriter    io.Writer
	CacheDir          string
}

func ListAvailableVersions(ctx context.Context, opts InstallOptions) ([]AvailableVersion, error) {
	repo := opts.Repo
	if len(repo.Repos) == 0 {
		var err error
		repo, err = GetDefaultRepo()
		if err != nil {
			return nil, err
		}
	}
	fetcher, err := NewFetcher(repo, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, err
	}
	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}

	var lastErr error
	for _, githubRepo := range repo.Repos {
		releases, err := getReleases(ctx, client, githubRepo)
		if err != nil {
			lastErr = err
			continue
		}
		versions := make([]AvailableVersion, 0)
		for _, release := range releases {
			if release.Prerelease && !opts.IncludePrerelease {
				continue
			}
			for _, asset := range release.Assets {
				version, err := fetcher.CheckAsset(asset, release)
				if err != nil {
					return nil, err
				}
				if version != nil {
					version.Repo = githubRepo
					versions = append(versions, *version)
				}
			}
		}
		if len(versions) > 0 {
			return versions, nil
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("no matching releases found for %s %s", fetcher.OS, fetcher.Arch)
}

func Install(ctx context.Context, opts InstallOptions) (*InstalledVersion, error) {
	versions, err := ListAvailableVersions(ctx, InstallOptions{
		Repo:              opts.Repo,
		Client:            opts.Client,
		IncludePrerelease: true,
	})
	if err != nil {
		return nil, err
	}
	selected, err := selectVersion(versions, opts.VersionSpec)
	if err != nil {
		return nil, err
	}

	cacheDir := opts.CacheDir
	if cacheDir == "" {
		var err error
		cacheDir, err = CacheDir()
		if err != nil {
			return nil, err
		}
	}
	root := browsersDir(cacheDir)
	repoDir := safeVersionDir(selected.RepoName)
	if repoDir == "" {
		repoDir = "default"
	}
	installDir := filepath.Join(root, repoDir, safeVersionDir(selected.Version.FullString()))
	if opts.Replace {
		if err := os.RemoveAll(installDir); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filepath.Join(installDir, "version.json")); err == nil && !opts.Replace {
		launch, launchErr := LaunchPath(installDir)
		if launchErr != nil {
			return nil, launchErr
		}
		if err := VerifyLaunchPath(launch); err != nil {
			return nil, err
		}
		return &InstalledVersion{
			Path:      installDir,
			Version:   selected.Version,
			Repo:      selected.RepoName,
			Channel:   selected.Version.Build,
			LaunchExe: launch,
		}, nil
	}

	tmp, err := os.CreateTemp("", "go-camoufox-*.zip")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	defer tmp.Close()

	client := opts.Client
	if client == nil {
		client = http.DefaultClient
	}
	if err := download(ctx, client, selected.URL, tmp, opts.DownloadWriter); err != nil {
		return nil, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return nil, err
	}
	if err := unzip(tmpName, installDir); err != nil {
		_ = os.RemoveAll(installDir)
		return nil, err
	}
	if err := WriteVersionMetadata(installDir, selected); err != nil {
		return nil, err
	}
	launch, err := LaunchPath(installDir)
	if err != nil {
		return nil, err
	}
	if err := VerifyLaunchPath(launch); err != nil {
		return nil, err
	}
	return &InstalledVersion{
		Path:      installDir,
		Version:   selected.Version,
		Repo:      selected.RepoName,
		Channel:   selected.Version.Build,
		LaunchExe: launch,
	}, nil
}

func getReleases(ctx context.Context, client HTTPDoer, githubRepo string) ([]Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/"+githubRepo+"/releases", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("github releases %s: %s: %s", githubRepo, resp.Status, strings.TrimSpace(string(body)))
	}
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	return releases, nil
}

func download(ctx context.Context, client HTTPDoer, url string, dst io.Writer, progress io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	written, err := io.Copy(dst, resp.Body)
	if err != nil {
		return err
	}
	if progress != nil {
		_, _ = fmt.Fprintf(progress, "downloaded %d bytes\n", written)
	}
	return nil
}

func unzip(zipPath, dest string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer reader.Close()

	destClean, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	for _, file := range reader.File {
		target := filepath.Join(destClean, file.Name)
		targetClean, err := filepath.Abs(target)
		if err != nil {
			return err
		}
		if targetClean != destClean && !strings.HasPrefix(targetClean, destClean+string(os.PathSeparator)) {
			return fmt.Errorf("zip entry escapes destination: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetClean, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetClean), 0o755); err != nil {
			return err
		}
		src, err := file.Open()
		if err != nil {
			return err
		}
		err = writeZipFile(targetClean, src, file.Mode())
		closeErr := src.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func writeZipFile(path string, src io.Reader, mode os.FileMode) error {
	dst, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

func selectVersion(versions []AvailableVersion, spec string) (AvailableVersion, error) {
	if len(versions) == 0 {
		return AvailableVersion{}, fmt.Errorf("no versions available")
	}
	spec = strings.TrimPrefix(strings.TrimSpace(spec), "v")
	if spec == "" {
		return versions[0], nil
	}
	for _, version := range versions {
		if version.Version.FullString() == spec || version.Version.Build == spec {
			return version, nil
		}
	}
	return AvailableVersion{}, fmt.Errorf("version %q not found", spec)
}

func safeVersionDir(value string) string {
	value = strings.TrimPrefix(value, "v")
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_")
	return replacer.Replace(value)
}
