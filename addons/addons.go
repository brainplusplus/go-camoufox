package addons

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	camoufox "github.com/brainplusplus/go-camoufox/internal/types"
)

const UBOURL = "https://addons.mozilla.org/firefox/downloads/latest/ublock-origin/latest.xpi"

var DefaultAddonURLs = map[camoufox.DefaultAddon]string{
	camoufox.AddonUBO: UBOURL,
}

var DefaultAddonNames = map[camoufox.DefaultAddon]string{
	camoufox.AddonUBO: "UBO",
}

func AddDefaultAddons(addons *[]string, exclude []camoufox.DefaultAddon) error {
	excluded := make(map[camoufox.DefaultAddon]struct{}, len(exclude))
	for _, item := range exclude {
		excluded[item] = struct{}{}
	}
	for addon, url := range DefaultAddonURLs {
		if _, skip := excluded[addon]; skip {
			continue
		}
		name := DefaultAddonNames[addon]
		path, err := GetAddonPath(name)
		if err != nil {
			return err
		}
		if err := ensureAddon(url, path, name); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to download default addon %s: %v\n", name, err)
			continue
		}
		*addons = append(*addons, path)
	}
	return nil
}

func ConfirmPaths(paths []string) error {
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("addon path %q is not accessible: %w", path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("addon path %q must be an extracted addon directory", path)
		}
		if _, err := os.Stat(filepath.Join(path, "manifest.json")); err != nil {
			return fmt.Errorf("addon path %q is missing manifest.json: %w", path, err)
		}
	}
	return nil
}

func DownloadAndExtract(url, extractPath, name string) error {
	if err := os.MkdirAll(extractPath, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp("", "go-camoufox-addon-*.xpi")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	defer tmp.Close()

	resp, err := http.Get(url) //nolint:gosec // User-visible addon URL mirrors upstream Camoufox behavior.
	if err != nil {
		return fmt.Errorf("download addon %s: %w", name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download addon %s: %s", name, resp.Status)
	}
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := unzip(tmpName, extractPath); err != nil {
		return fmt.Errorf("extract addon %s: %w", name, err)
	}
	return ConfirmPaths([]string{extractPath})
}

func GetAddonPath(addonName string) (string, error) {
	dir, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "addons", addonName), nil
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

func ensureAddon(url, path, name string) error {
	if _, err := os.Stat(filepath.Join(path, "manifest.json")); err == nil {
		return nil
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return DownloadAndExtract(url, path, name)
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
