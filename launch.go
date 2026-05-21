package camoufox

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/brainplusplus/go-camoufox/addons"
	"github.com/brainplusplus/go-camoufox/fingerprint"
	"github.com/brainplusplus/go-camoufox/geolocation"
	internalconfig "github.com/brainplusplus/go-camoufox/internal/config"
	"github.com/brainplusplus/go-camoufox/pkgman"
	"github.com/brainplusplus/go-camoufox/virtdisplay"
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

const defaultFirefoxMajor = 150

var (
	publicIP       = geolocation.PublicIP
	getGeolocation = geolocation.GetGeolocation
)

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

	var virtualDisplay interface{ Close() error }
	if headlessMode == HeadlessVirtual {
		display := ""
		if opts.VirtualDisplay != nil {
			display = *opts.VirtualDisplay
		}
		debug := boolValue(opts.Debug)
		vd, err := virtdisplay.New(display, debug)
		if err != nil {
			return nil, err
		}
		virtualDisplay = vd
		virtdisplay.ApplyEnv(env, vd.Display)
		headlessMode = HeadlessFalse
	}

	if opts.VirtualDisplay != nil && *opts.VirtualDisplay != "" {
		virtdisplay.ApplyEnv(env, *opts.VirtualDisplay)
	}
	fail := func(err error) (*BuiltLaunchOptions, error) {
		if virtualDisplay != nil {
			_ = virtualDisplay.Close()
		}
		return nil, err
	}

	if err := checkValidOS(opts.OS); err != nil {
		return fail(err)
	}
	if len(opts.OS) == 0 && opts.WebGLConfig != nil {
		return fail(errors.New("os must be set when using webgl_config"))
	}

	executablePath := opts.ExecutablePath
	if executablePath == "" {
		if opts.Browser != nil && *opts.Browser != "" {
			executablePath, err = pkgman.FindInstalledLaunchPath(*opts.Browser)
		} else {
			executablePath, err = pkgman.LaunchPath("")
		}
		if err != nil {
			return fail(err)
		}
	}

	iKnow := opts.IKnowWhatImDoing != nil && *opts.IKnowWhatImDoing
	if !iKnow {
		warnings.WarnManualConfig(config)
	}

	if opts.FFVersion != nil {
		warnings.Warn("ff_version", iKnow)
	}
	ffVersion := resolveFirefoxMajor(opts.FFVersion, executablePath)

	addonList := append([]string(nil), opts.Addons...)
	if err := addons.AddDefaultAddons(&addonList, opts.ExcludeAddons); err != nil {
		return fail(err)
	}
	if len(addonList) > 0 {
		if err := addons.ConfirmPaths(addonList); err != nil {
			return fail(err)
		}
		config["addons"] = addonList
	}

	if opts.Fonts != nil {
		config["fonts"] = append([]string(nil), opts.Fonts...)
	}
	if boolValue(opts.CustomFontsOnly) {
		firefoxPrefs["gfx.bundled-fonts.activate"] = 0
		if len(opts.Fonts) == 0 {
			return fail(errors.New("no custom fonts were passed, but custom_fonts_only is enabled"))
		}
		warnings.Warn("custom_fonts_only", false)
	}

	if opts.Fingerprint != nil && opts.Fingerprint.Screen != nil && opts.Screen == nil {
		optsCopy := *opts
		optsCopy.Screen = opts.Fingerprint.Screen
		opts = &optsCopy
	}

	if err := applyFingerprintOptions(config, opts, ffVersion, iKnow); err != nil {
		return fail(err)
	}

	if opts.GeoIP != nil {
		if err := applyGeoIP(config, firefoxPrefs, opts); err != nil {
			return fail(err)
		}
	} else if opts.Proxy != nil && !strings.Contains(opts.Proxy.Server, "localhost") && !isDomainSet(config, "geolocation") {
		warnings.Warn("proxy_without_geoip", false)
	}

	if len(opts.Locale) > 0 {
		if err := geolocation.HandleLocales(opts.Locale, config); err != nil {
			return fail(err)
		}
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
	if boolValue(opts.BlockWebGL) || opts.AllowWebGL != nil && !*opts.AllowWebGL {
		warnings.Warn("block_webgl", iKnow)
		firefoxPrefs["webgl.disabled"] = true
	} else {
		targetOS := fingerprint.TargetOSFromConfig(config)
		var vendor, renderer string
		if opts.WebGLConfig != nil {
			vendor, renderer = opts.WebGLConfig[0], opts.WebGLConfig[1]
		} else {
			vendor, _ = config["webGl:vendor"].(string)
			renderer, _ = config["webGl:renderer"].(string)
		}
		if webgl, err := fingerprint.SampleWebGL(targetOS, vendor, renderer, nil); err == nil {
			enableWebGL2, _ := webgl["webGl2Enabled"].(bool)
			delete(webgl, "webGl2Enabled")
			fingerprint.MergeInto(config, webgl)
			fingerprint.MergeInto(firefoxPrefs, map[string]any{
				"webgl.enable-webgl2": enableWebGL2,
				"webgl.force-enabled": true,
			})
		} else if opts.WebGLConfig != nil {
			return fail(err)
		}
	}
	if boolValue(opts.EnableCache) {
		for key, value := range CachePrefs {
			firefoxPrefs[key] = value
		}
	}

	if err := internalconfig.Validate(config); err != nil {
		return fail(err)
	}

	camouEnv, err := CamouConfigEnv(config, runtime.GOOS)
	if err != nil {
		return fail(err)
	}
	for key, value := range camouEnv {
		if _, exists := env[key]; !exists {
			env[key] = value
		}
	}
	if err := applyFontConfigEnv(env, executablePath, fingerprint.TargetOSFromConfig(config), runtime.GOOS); err != nil {
		return fail(err)
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
		VirtualDisplay:   virtualDisplay,
	}, nil
}

func applyGeoIP(config, firefoxPrefs map[string]any, opts *LaunchOptions) error {
	ip := opts.GeoIP.IP
	if opts.GeoIP.Auto {
		proxyString := ""
		if opts.Proxy != nil {
			var err error
			proxyString, err = (geolocation.Proxy{
				Server:   opts.Proxy.Server,
				Username: opts.Proxy.Username,
				Password: opts.Proxy.Password,
				Bypass:   opts.Proxy.Bypass,
			}).AsString()
			if err != nil {
				return err
			}
		}
		resolved, err := publicIP(context.Background(), proxyString)
		if err != nil {
			return err
		}
		ip = resolved
	}
	if ip == "" {
		return errors.New("geoip requires either Auto or an IP address")
	}
	if !boolValue(opts.BlockWebRTC) {
		if geolocation.ValidIPv4(ip) {
			fingerprint.SetInto(config, "webrtc:ipv4", ip)
			firefoxPrefs["network.dns.disableIPv6"] = true
		} else if geolocation.ValidIPv6(ip) {
			fingerprint.SetInto(config, "webrtc:ipv6", ip)
		}
	}
	geoipDB := ""
	if opts.GeoIPDB != nil {
		geoipDB = *opts.GeoIPDB
	}
	geo, err := getGeolocation(context.Background(), ip, geoipDB)
	if err != nil {
		return err
	}
	for key, value := range geo.AsConfig() {
		switch key {
		case "timezone", "locale:language", "locale:region", "locale:script":
			fingerprint.SetInto(config, key, value)
		default:
			config[key] = value
		}
	}
	return nil
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

func applyFontConfigEnv(env map[string]string, executablePath, targetOS, goos string) error {
	if goos != "linux" {
		return nil
	}
	if _, exists := env["FONTCONFIG_FILE"]; exists {
		return nil
	}
	fontconfigPath, ok := findFontConfig(executablePath, targetOS)
	if !ok {
		return fmt.Errorf("fonts.conf not found for target OS %q near %q", targetOS, executablePath)
	}
	runtimePath, err := runtimeFontConfig(fontconfigPath, executablePath)
	if err != nil {
		return err
	}
	env["FONTCONFIG_FILE"] = runtimePath
	return nil
}

func findFontConfig(executablePath, targetOS string) (string, bool) {
	if executablePath == "" {
		return "", false
	}
	base := filepath.Dir(executablePath)
	candidates := []string{
		filepath.Join(base, "fontconfigs", targetOS, "fonts.conf"),
		filepath.Join(base, "fontconfig", targetOS, "fonts.conf"),
		filepath.Join(base, "fontconfigs", "fonts.conf"),
		filepath.Join(base, "fontconfig", "fonts.conf"),
		filepath.Join(base, "browser", "fontconfigs", targetOS, "fonts.conf"),
		filepath.Join(base, "browser", "fontconfig", targetOS, "fonts.conf"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
	}
	return "", false
}

func runtimeFontConfig(fontsConfPath, executablePath string) (string, error) {
	data, err := os.ReadFile(fontsConfPath)
	if err != nil {
		return "", err
	}
	fontsDir := filepath.Join(filepath.Dir(executablePath), "fonts")
	content := strings.ReplaceAll(string(data), `<dir prefix="cwd">fonts</dir>`, `<dir>`+fontsDir+`</dir>`)
	cacheDir, err := pkgman.CacheDir()
	if err != nil {
		return "", err
	}
	targetDir := filepath.Join(cacheDir, "fontconfig")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(content))
	target := filepath.Join(targetDir, fmt.Sprintf("fonts-%x.conf", sum[:6]))
	if _, err := os.Stat(target); os.IsNotExist(err) {
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}
	return target, nil
}

func checkValidOS(values []string) error {
	for _, value := range values {
		if _, ok := validFingerprintOS[value]; !ok {
			return fmt.Errorf("invalid os %q: expected one of linux, macos, windows", value)
		}
	}
	return nil
}

func applyFingerprintOptions(config map[string]any, opts *LaunchOptions, ffVersion int, iKnow bool) error {
	usedPreset := false
	ffVersionStr := strconv.Itoa(ffVersion)
	if opts.Fingerprint != nil {
		if !iKnow && opts.Fingerprint.UserAgent != "" && !strings.Contains(opts.Fingerprint.UserAgent, "Firefox") {
			return fmt.Errorf("%q fingerprints are not supported in Camoufox", opts.Fingerprint.UserAgent)
		}
		if !iKnow {
			warnings.Warn("custom_fingerprint", false)
		}
		if opts.Fingerprint.Raw != nil {
			fp, err := fingerprint.FromBrowserForge(&fingerprint.BrowserForgeFingerprint{Raw: opts.Fingerprint.Raw}, ffVersionStr)
			if err != nil {
				return err
			}
			fingerprint.MergeInto(config, fp)
		}
		if opts.Fingerprint.UserAgent != "" {
			fingerprint.SetInto(config, "navigator.userAgent", opts.Fingerprint.UserAgent)
		}
	} else if opts.FingerprintPreset != nil {
		var preset map[string]any
		var err error
		if opts.FingerprintPreset.Preset != nil {
			preset = opts.FingerprintPreset.Preset
		} else if opts.FingerprintPreset.UseRandom {
			preset, err = fingerprint.GetRandomPreset(opts.OS, ffVersionStr, nil)
			if err != nil {
				return err
			}
		}
		if preset != nil {
			fp, err := fingerprint.FromPreset(preset, ffVersionStr, nil)
			if err != nil {
				return err
			}
			fingerprint.MergeInto(config, fp)
			usedPreset = true
		}
	}

	if !usedPreset && opts.Fingerprint == nil {
		fp, err := fingerprint.GenerateFingerprint(opts.OS, toFingerprintScreenConstraint(opts.Screen), opts.Window, ffVersionStr, nil)
		if err != nil {
			return err
		}
		converted, err := fingerprint.FromBrowserForge(fp, ffVersionStr)
		if err != nil {
			return err
		}
		fingerprint.MergeInto(config, converted)
	}

	targetOS := fingerprint.TargetOSFromConfig(config)
	fingerprint.SetInto(config, "window.history.length", randomRange(1, 6))
	if _, ok := config["fonts"]; !ok && !boolValue(opts.CustomFontsOnly) {
		if fonts, err := fingerprint.GenerateRandomFontSubset(targetOS, nil); err == nil {
			config["fonts"] = fonts
		}
	}
	if _, ok := config["voices"]; !ok {
		if voices, err := fingerprint.GenerateRandomVoiceSubset(targetOS, nil); err == nil {
			config["voices"] = voices
		}
	}
	fingerprint.SetInto(config, "fonts:spacing_seed", randomSeed())
	fingerprint.SetInto(config, "audio:seed", randomSeed())
	fingerprint.SetInto(config, "canvas:seed", randomSeed())
	return nil
}

func toFingerprintScreenConstraint(screen *ScreenConstraint) *fingerprint.ScreenConstraint {
	if screen == nil {
		return nil
	}
	return &fingerprint.ScreenConstraint{
		MinWidth:  screen.MinWidth,
		MaxWidth:  screen.MaxWidth,
		MinHeight: screen.MinHeight,
		MaxHeight: screen.MaxHeight,
	}
}

func resolveFirefoxMajor(explicit *int, executablePath string) int {
	if explicit != nil {
		return *explicit
	}
	if major, ok := installedFirefoxMajor(executablePath); ok {
		return major
	}
	return defaultFirefoxMajor
}

func installedFirefoxMajor(executablePath string) (int, bool) {
	if executablePath == "" {
		return 0, false
	}
	target, err := filepath.Abs(executablePath)
	if err != nil {
		return 0, false
	}
	target = filepath.Clean(target)
	installed, err := pkgman.ListInstalled()
	if err != nil {
		return 0, false
	}
	for _, item := range installed {
		launch, err := filepath.Abs(item.LaunchExe)
		if err != nil {
			continue
		}
		if !samePath(filepath.Clean(launch), target) {
			continue
		}
		major, err := strconv.Atoi(strings.SplitN(item.Version.Version, ".", 2)[0])
		if err == nil && major > 0 {
			return major, true
		}
	}
	return 0, false
}

func samePath(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func randomRange(minInclusive, maxExclusive int) int {
	var value int
	_ = fingerprint.WithGlobalRNG(func(rng fingerprint.RNG) error {
		value = minInclusive + rng.Intn(maxExclusive-minInclusive)
		return nil
	})
	return value
}

func randomSeed() uint32 {
	var value uint32
	_ = fingerprint.WithGlobalRNG(func(rng fingerprint.RNG) error {
		for value == 0 {
			value = rng.Uint32()
		}
		return nil
	})
	return value
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
