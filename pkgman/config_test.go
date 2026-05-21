package pkgman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigActiveSetAndRemove(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("GO_CAMOUFOX_CACHE", cache)
	installed := fakeInstalled(t, cache, "Official", "135.0.1", "beta.24")

	config, err := SetPinned("official/prerelease/135.0.1-beta.24")
	if err != nil {
		t.Fatal(err)
	}
	if config.ActiveVersion != "official/135.0.1-beta.24" {
		t.Fatalf("expected installed pinned version to become active: %#v", config)
	}
	display, fetched, err := ActiveDisplay()
	if err != nil {
		t.Fatal(err)
	}
	if !fetched || display != "official/prerelease/135.0.1-beta.24" {
		t.Fatalf("unexpected active display: %q fetched=%v", display, fetched)
	}
	if path, err := LaunchPath(""); err != nil || path != installed.LaunchExe {
		t.Fatalf("expected active launch path %q, got %q err=%v", installed.LaunchExe, path, err)
	}
	removed, err := RemoveInstalled("official/prerelease/135.0.1-beta.24")
	if err != nil {
		t.Fatal(err)
	}
	if removed.Version.FullString() != "135.0.1-beta.24" {
		t.Fatalf("unexpected removed version: %#v", removed)
	}
	config, err = LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.ActiveVersion != "" {
		t.Fatalf("active version was not cleared: %#v", config)
	}
}

func TestSetChannelWithoutInstalledVersion(t *testing.T) {
	t.Setenv("GO_CAMOUFOX_CACHE", t.TempDir())
	config, err := SetChannel("official/stable")
	if err != nil {
		t.Fatal(err)
	}
	if config.Channel != "official/stable" || config.Pinned != "" || config.ActiveVersion != "" {
		t.Fatalf("unexpected channel config: %#v", config)
	}
	display, fetched, err := ActiveDisplay()
	if err != nil {
		t.Fatal(err)
	}
	if fetched || display != "official/stable" {
		t.Fatalf("unexpected active display: %q fetched=%v", display, fetched)
	}
}

func fakeInstalled(t *testing.T, cache, repo, version, build string) InstalledVersion {
	t.Helper()
	dir := filepath.Join(cache, "browsers", repo, version+"-"+build)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	launchFile, err := LaunchFile("windows")
	if err != nil {
		t.Fatal(err)
	}
	launch := filepath.Join(dir, launchFile)
	if err := os.WriteFile(launch, []byte("fake"), 0o755); err != nil {
		t.Fatal(err)
	}
	metadata := []byte(`{"version":"` + version + `","build":"` + build + `","repo_name":"` + repo + `"}`)
	if err := os.WriteFile(filepath.Join(dir, "version.json"), metadata, 0o644); err != nil {
		t.Fatal(err)
	}
	return InstalledVersion{
		Path:      dir,
		Version:   Version{Version: version, Build: build},
		Repo:      repo,
		Channel:   build,
		LaunchExe: launch,
	}
}
