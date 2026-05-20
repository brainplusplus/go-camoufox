package addons

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	camoufox "github.com/brainplusplus/go-camoufox/internal/types"
)

func TestDownloadAndExtractValidatesManifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(makeZip(t, map[string]string{
			"manifest.json": `{"manifest_version":2}`,
			"background.js": "void 0",
		}))
	}))
	defer server.Close()

	target := filepath.Join(t.TempDir(), "UBO")
	if err := DownloadAndExtract(server.URL, target, "UBO"); err != nil {
		t.Fatal(err)
	}
	if err := ConfirmPaths([]string{target}); err != nil {
		t.Fatal(err)
	}
}

func TestConfirmPathsRequiresExtractedAddonDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := ConfirmPaths([]string{dir}); err == nil {
		t.Fatal("expected missing manifest error")
	}
}

func TestAddDefaultAddonsUsesCacheAndExclude(t *testing.T) {
	oldURLs := DefaultAddonURLs
	DefaultAddonURLs = map[camoufox.DefaultAddon]string{
		camoufox.AddonUBO: "unused",
	}
	defer func() { DefaultAddonURLs = oldURLs }()

	t.Setenv("GO_CAMOUFOX_CACHE", t.TempDir())
	addonPath, err := GetAddonPath("UBO")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(addonPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(addonPath, "manifest.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	var paths []string
	if err := AddDefaultAddons(&paths, nil); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != addonPath {
		t.Fatalf("unexpected addon paths: %#v", paths)
	}

	paths = nil
	if err := AddDefaultAddons(&paths, []camoufox.DefaultAddon{camoufox.AddonUBO}); err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected excluded addon, got %#v", paths)
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
