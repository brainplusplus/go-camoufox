package camoufox

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/brainplusplus/go-camoufox/addons"
	internalconfig "github.com/brainplusplus/go-camoufox/internal/config"
	"github.com/brainplusplus/go-camoufox/pkgman"
	"github.com/brainplusplus/go-camoufox/warnings"
)

var CachePrefs = map[string]any{
	"browser.sessionhistory.max_entries":       10,
	"browser.sessionhistory.max_total_viewers": -1,
	"browser.cache.memory.enable":              true,
	"browser.cache.disk_cache_ssl":             true,
	"browser.cache.disk.smart_size.enabled":    true,
}

var validFingerprintOS = map[string]struct{}{
	"windows": {},
	"macos":   {},
	"linux":   {},
}

func BuildLaunchOptions(opts *LaunchOptions) (*BuiltLaunchOptions, error) {
	if opts == nil {
		opts = &LaunchOptions{}
	}

	config := cloneAnyMap(opts.Config)
	firefoxPrefs := cloneAnyMap(opts.FirefoxUserPrefs)
	args := append([]string(nil), opts.Args...)
	env, err := normalizeEnv(opts.Env)
	if err != nil {
		return nil, err
	}
	if env == nil {
		env = currentEnv()
	}

	headlessMode := HeadlessFalse
	if opts.Headless != nil {
		headlessMode = *opts.Headless
	}

	if opts.VirtualDisplay != nil && *opts.VirtualDisplay != "" {
		env["DISPLAY"] = *opts.VirtualDisplay
		env["GDK_BACKEND"] = "x11"
		delete(env, "WAYLAND_DISPLAY")
		env["MOZ_ENABLE_WAYLAND"] = "0"
	}

	if err := checkValidOS(opts.OS); err != nil {
		return nil, err
	}
	if len(opts.OS) == 0 && opts.WebGLConfig != nil {
		return nil, errors.New("os must be set when using webgl_config")
	}

	iKnow := opts.IKnowWhatImDoing != nil && *opts.IKnowWhatImDoing
	if !iKnow {
		warnings.WarnManualConfig(config)
	}

	addonList := append([]string(nil), opts.Addons...)
	if err := addons.AddDefaultAddons(&addonList, opts.ExcludeAddons); err != nil {
		return nil, err
	}
	if len(addonList) > 0 {
		if err := addons.ConfirmPaths(addonList); err != nil {
			return nil, err
		}
		config["addons"] = addonList
	}

	if opts.Fonts != nil {
		config["fonts"] = append([]string(nil), opts.Fonts...)
	}
	if boolValue(opts.CustomFontsOnly) {
		firefoxPrefs["gfx.bundled-fonts.activate"] = 0
		if len(opts.Fonts) == 0 {
			return nil, errors.New("no custom fonts were passed, but custom_fonts_only is enabled")
		}
		warnings.Warn("custom_fonts_only", iKnow)
	}

	if opts.GeoIP == nil && opts.Proxy != nil && !strings.Contains(opts.Proxy.Server, "localhost") && !isDomainSet(config, "geolocation") {
		warnings.Warn("proxy_without_geoip", false)
	}

	if opts.Humanize != nil && opts.Humanize.Enabled {
		config["humanize"] = true
		if opts.Humanize.MaxTime != nil {
			config["humanize:maxTime"] = *opts.Humanize.MaxTime
		}
	}
	if boolValue(opts.MainWorldEval) {
		config["allowMainWorld"] = true
	}

	if boolValue(opts.BlockImages) {
		warnings.Warn("block_images", iKnow)
		firefoxPrefs["permissions.default.image"] = 2
	}
	if boolValue(opts.BlockWebRTC) {
		firefoxPrefs["media.peerconnection.enabled"] = false
	}
	if boolValue(opts.DisableCOOP) {
		warnings.Warn("disable_coop", iKnow)
		firefoxPrefs["browser.tabs.remote.useCrossOriginOpenerPolicy"] = false
	}
	if boolValue(opts.BlockWebGL) {
		warnings.Warn("block_webgl", iKnow)
		firefoxPrefs["webgl.disabled"] = true
	}
	if boolValue(opts.EnableCache) {
		for key, value := range CachePrefs {
			firefoxPrefs[key] = value
		}
	}

	if err := internalconfig.Validate(config); err != nil {
		return nil, err
	}

	camouEnv, err := CamouConfigEnv(config, runtime.GOOS)
	if err != nil {
		return nil, err
	}
	for key, value := range camouEnv {
		env[key] = value
	}

	executablePath := opts.ExecutablePath
	if executablePath == "" {
		if opts.Browser != nil && *opts.Browser != "" {
			executablePath, err = pkgman.FindInstalledLaunchPath(*opts.Browser)
		} else {
			executablePath, err = pkgman.LaunchPath("")
		}
		if err != nil {
			return nil, err
		}
	}

	return &BuiltLaunchOptions{
		ExecutablePath:   executablePath,
		Args:             args,
		Env:              env,
		FirefoxUserPrefs: firefoxPrefs,
		Headless:         headlessMode == HeadlessTrue,
		Proxy:            opts.Proxy,
		Extra:            cloneAnyMap(opts.Extra),
		Config:           config,
	}, nil
}

func CamouConfigEnv(config map[string]any, goos string) (map[string]string, error) {
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("encode camoufox config: %w", err)
	}
	chunkSize := 32767
	if goos == "windows" {
		chunkSize = 2047
	}
	configStr := string(data)
	if configStr == "" {
		configStr = "{}"
	}

	out := make(map[string]string)
	for i, n := 0, 1; i < len(configStr); i, n = i+chunkSize, n+1 {
		end := i + chunkSize
		if end > len(configStr) {
			end = len(configStr)
		}
		out[fmt.Sprintf("CAMOU_CONFIG_%d", n)] = configStr[i:end]
	}
	return out, nil
}

func checkValidOS(values []string) error {
	for _, value := range values {
		if _, ok := validFingerprintOS[value]; !ok {
			return fmt.Errorf("invalid os %q: expected one of linux, macos, windows", value)
		}
	}
	return nil
}

func boolValue(value *bool) bool {
	return value != nil && *value
}

func cloneAnyMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func normalizeEnv(input map[string]any) (map[string]string, error) {
	if input == nil {
		return nil, nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			out[key] = typed
		case fmt.Stringer:
			out[key] = typed.String()
		case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			out[key] = fmt.Sprint(typed)
		default:
			return nil, fmt.Errorf("env %s has unsupported value type %T", key, value)
		}
	}
	return out, nil
}

func currentEnv() map[string]string {
	env := make(map[string]string)
	for _, pair := range os.Environ() {
		key, value, ok := strings.Cut(pair, "=")
		if ok {
			env[key] = value
		}
	}
	return env
}

func isDomainSet(config map[string]any, domain string) bool {
	prefix := domain + ":"
	for key := range config {
		if key == domain || strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

func StableConfigJSON(config map[string]any) (string, error) {
	keys := make([]string, 0, len(config))
	for key := range config {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := make(map[string]any, len(config))
	for _, key := range keys {
		ordered[key] = config[key]
	}
	data, err := json.Marshal(ordered)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
