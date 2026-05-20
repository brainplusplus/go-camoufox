package pkgman

import "testing"

func TestMapOSAndArch(t *testing.T) {
	tests := []struct {
		goos, goarch string
		wantOS       string
		wantArch     string
	}{
		{"linux", "amd64", "lin", "x86_64"},
		{"linux", "arm64", "lin", "arm64"},
		{"darwin", "amd64", "mac", "x86_64"},
		{"darwin", "arm64", "mac", "arm64"},
		{"windows", "amd64", "win", "x86_64"},
	}
	for _, tt := range tests {
		gotOS, err := MapOS(tt.goos)
		if err != nil {
			t.Fatal(err)
		}
		gotArch, err := MapArch(tt.goarch)
		if err != nil {
			t.Fatal(err)
		}
		if gotOS != tt.wantOS || gotArch != tt.wantArch {
			t.Fatalf("%s/%s got %s/%s", tt.goos, tt.goarch, gotOS, gotArch)
		}
	}
}

func TestLoadReposAndAssetMatch(t *testing.T) {
	repos, err := LoadRepos("0.5.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) < 2 {
		t.Fatalf("expected repos from embedded config, got %d", len(repos))
	}
	fetcher, err := NewFetcher(repos[0], "windows", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	version, err := fetcher.CheckAsset(ReleaseAsset{
		Name:               "camoufox-150.0.2-beta.25-win.x86_64.zip",
		BrowserDownloadURL: "https://example.invalid/camoufox.zip",
	}, Release{Prerelease: true})
	if err != nil {
		t.Fatal(err)
	}
	if version == nil {
		t.Fatal("expected matching version")
	}
	if version.Version.Build != "beta.25" || version.Version.Version != "150.0.2" {
		t.Fatalf("unexpected version: %#v", version.Version)
	}
}
