package pkgman

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) Do(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestListAvailableVersionsFromHTTPFixture(t *testing.T) {
	client := fixtureClient(t, nil)
	repo := RepoConfig{
		Repos:    []string{"example/camoufox"},
		Name:     "Fixture",
		Pattern:  "{name}-{version}-{build}-{os}.{arch}.zip",
		BuildMin: "beta.19",
		BuildMax: "1",
	}
	versions, err := ListAvailableVersions(context.Background(), InstallOptions{
		Repo:              repo,
		Client:            client,
		IncludePrerelease: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	if versions[0].Version.FullString() != "150.0.2-beta.25" {
		t.Fatalf("unexpected version: %s", versions[0].Version.FullString())
	}
}

func TestInstallDownloadsExtractsAndWritesMetadata(t *testing.T) {
	launchFile, err := LaunchFile(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	zipBytes := makeZip(t, map[string]string{
		launchFile:    "fixture",
		"nested/file": "ok",
	})
	client := fixtureClient(t, zipBytes)
	cacheDir := t.TempDir()
	repo := RepoConfig{
		Repos:    []string{"example/camoufox"},
		Name:     "Fixture",
		Pattern:  "{name}-{version}-{build}-{os}.{arch}.zip",
		BuildMin: "beta.19",
		BuildMax: "1",
	}
	installed, err := Install(context.Background(), InstallOptions{
		Repo:        repo,
		Client:      client,
		CacheDir:    cacheDir,
		VersionSpec: "150.0.2-beta.25",
	})
	if err != nil {
		t.Fatal(err)
	}
	if installed.Version.FullString() != "150.0.2-beta.25" {
		t.Fatalf("unexpected installed version: %s", installed.Version.FullString())
	}
	if _, err := os.Stat(filepath.Join(installed.Path, launchFile)); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filepath.Dir(installed.Path)) != "Fixture" {
		t.Fatalf("expected repo-scoped install path, got %s", installed.Path)
	}
	version, err := VersionFromPath(filepath.Join(installed.Path, "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	if version.Build != "beta.25" || version.Version != "150.0.2" {
		t.Fatalf("unexpected metadata: %#v", version)
	}
	metadata, err := ReadVersionMetadata(filepath.Join(installed.Path, "version.json"))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.Repo != "example/camoufox" || metadata.RepoName != "Fixture" || metadata.AssetID != 42 {
		t.Fatalf("metadata did not preserve repo fields: %#v", metadata)
	}
}

func fixtureClient(t *testing.T, zipBytes []byte) roundTripFunc {
	t.Helper()
	osName, err := MapOS(runtime.GOOS)
	if err != nil {
		t.Fatal(err)
	}
	arch, err := MapArch(runtime.GOARCH)
	if err != nil {
		t.Fatal(err)
	}
	releases := []Release{
		{
			Prerelease: true,
			Assets: []ReleaseAsset{
				{
					ID:                 42,
					Name:               "camoufox-150.0.2-beta.25-" + osName + "." + arch + ".zip",
					BrowserDownloadURL: "https://downloads.example/camoufox.zip",
					Size:               int64(len(zipBytes)),
				},
			},
		},
	}
	releaseJSON, err := json.Marshal(releases)
	if err != nil {
		t.Fatal(err)
	}
	return func(req *http.Request) (*http.Response, error) {
		switch {
		case strings.Contains(req.URL.Host, "api.github.com"):
			return response(200, releaseJSON), nil
		case req.URL.String() == "https://downloads.example/camoufox.zip":
			return response(200, zipBytes), nil
		default:
			return response(404, []byte("not found")), nil
		}
	}
}

func response(status int, body []byte) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Header:     make(http.Header),
	}
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, content); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
