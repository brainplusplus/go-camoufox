package camoufox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/brainplusplus/go-camoufox/geolocation"
)

func TestCamouConfigEnvChunksWindows(t *testing.T) {
	config := map[string]any{"payload": strings.Repeat("x", 5000)}
	env, err := CamouConfigEnv(config, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if len(env) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(env))
	}
	if len(env["CAMOU_CONFIG_1"]) != 2047 {
		t.Fatalf("unexpected first chunk length: %d", len(env["CAMOU_CONFIG_1"]))
	}
	var decoded map[string]string
	joined := env["CAMOU_CONFIG_1"] + env["CAMOU_CONFIG_2"] + env["CAMOU_CONFIG_3"]
	if err := json.Unmarshal([]byte(joined), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["payload"] != strings.Repeat("x", 5000) {
		t.Fatal("reconstructed config did not match")
	}
}

func TestBuildLaunchOptionsFoundationalPrefs(t *testing.T) {
	blockImages := true
	blockWebGL := true
	enableCache := true
	headless := HeadlessTrue
	built, err := BuildLaunchOptions(&LaunchOptions{
		Config: map[string]any{
			"navigator.userAgent": "Mozilla/5.0",
		},
		BlockImages:      &blockImages,
		BlockWebGL:       &blockWebGL,
		EnableCache:      &enableCache,
		Headless:         &headless,
		ExecutablePath:   "C:/camoufox/camoufox.exe",
		FirefoxUserPrefs: map[string]any{"foo": "bar"},
		Env:              map[string]any{"BOOL": true, "FLOAT": 1.5},
		Proxy:            &ProxyConfig{Server: "socks5://127.0.0.1:1080"},
		IKnowWhatImDoing: boolPtr(true),
		CustomFontsOnly:  boolPtr(false),
		ExcludeAddons:    []DefaultAddon{AddonUBO},
		VirtualDisplay:   stringPtr(":99"),
		MainWorldEval:    boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !built.Headless {
		t.Fatal("expected headless true")
	}
	if built.Proxy == nil || built.Proxy.Server == "" {
		t.Fatal("expected proxy passthrough")
	}
	if built.FirefoxUserPrefs["permissions.default.image"] != 2 {
		t.Fatal("expected image blocking pref")
	}
	if built.FirefoxUserPrefs["webgl.disabled"] != true {
		t.Fatal("expected webgl disabled pref")
	}
	if built.FirefoxUserPrefs["browser.cache.memory.enable"] != true {
		t.Fatal("expected cache prefs")
	}
	if built.Env["DISPLAY"] != ":99" || built.Env["GDK_BACKEND"] != "x11" || built.Env["MOZ_ENABLE_WAYLAND"] != "0" {
		t.Fatalf("virtual display env not applied: %#v", built.Env)
	}
	if built.Config["allowMainWorld"] != true {
		t.Fatal("expected main world eval config")
	}
	if _, ok := built.Env["CAMOU_CONFIG_1"]; !ok {
		t.Fatal("missing CAMOU_CONFIG_1")
	}
	if built.Env["BOOL"] != "true" || built.Env["FLOAT"] != "1.5" {
		t.Fatalf("env values were not normalized: %#v", built.Env)
	}
}

func TestBuildLaunchOptionsRejectsInvalidOS(t *testing.T) {
	_, err := BuildLaunchOptions(&LaunchOptions{OS: []string{"beos"}, ExecutablePath: "camoufox"})
	if err == nil {
		t.Fatal("expected invalid os error")
	}
}

func TestBuildLaunchOptionsGeneratesEmbeddedFingerprint(t *testing.T) {
	built, err := BuildLaunchOptions(&LaunchOptions{
		OS:             []string{"windows"},
		ExecutablePath: "camoufox",
		ExcludeAddons:  []DefaultAddon{AddonUBO},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"navigator.userAgent",
		"navigator.platform",
		"screen.width",
		"webGl:vendor",
		"webGl:renderer",
		"fonts",
		"voices",
		"fonts:spacing_seed",
		"audio:seed",
		"canvas:seed",
		"window.history.length",
	} {
		if _, ok := built.Config[key]; !ok {
			t.Fatalf("generated config missing %s: %#v", key, built.Config)
		}
	}
	if built.FirefoxUserPrefs["webgl.force-enabled"] != true {
		t.Fatalf("expected webgl prefs to be merged: %#v", built.FirefoxUserPrefs)
	}
}

func TestBuildLaunchOptionsPresetAndExplicitWebGL(t *testing.T) {
	preset := map[string]any{
		"navigator": map[string]any{
			"userAgent":           "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0",
			"platform":            "Win32",
			"hardwareConcurrency": float64(12),
			"maxTouchPoints":      float64(0),
		},
		"screen": map[string]any{
			"width":       float64(1920),
			"height":      float64(1080),
			"colorDepth":  float64(24),
			"availWidth":  float64(1920),
			"availHeight": float64(1032),
		},
		"webgl": map[string]any{
			"unmaskedVendor":   "Google Inc. (Intel)",
			"unmaskedRenderer": "ANGLE (Intel, Intel(R) HD Graphics Direct3D11 vs_5_0 ps_5_0), or similar",
		},
	}
	ffVersion := 151
	webgl := [2]string{"Google Inc. (Intel)", "ANGLE (Intel, Intel(R) HD Graphics Direct3D11 vs_5_0 ps_5_0), or similar"}
	built, err := BuildLaunchOptions(&LaunchOptions{
		OS:                []string{"windows"},
		FFVersion:         &ffVersion,
		WebGLConfig:       &webgl,
		FingerprintPreset: &FingerprintPreset{Preset: preset},
		ExecutablePath:    "camoufox",
		ExcludeAddons:     []DefaultAddon{AddonUBO},
		IKnowWhatImDoing:  boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(built.Config["navigator.userAgent"].(string), "Firefox/151.0") {
		t.Fatalf("expected ff_version in preset UA: %v", built.Config["navigator.userAgent"])
	}
	if built.Config["webGl:vendor"] != webgl[0] || built.Config["webGl:renderer"] != webgl[1] {
		t.Fatalf("explicit webgl config not applied: %#v", built.Config)
	}
}

func TestBuildLaunchOptionsGeoIPAutoUsesProxyAndPrecedence(t *testing.T) {
	oldPublicIP := publicIP
	oldGetGeolocation := getGeolocation
	defer func() {
		publicIP = oldPublicIP
		getGeolocation = oldGetGeolocation
	}()
	var seenProxy string
	publicIP = func(ctx context.Context, proxy string) (string, error) {
		seenProxy = proxy
		return "8.8.8.8", nil
	}
	getGeolocation = func(ctx context.Context, ip, db string) (*geolocation.Geolocation, error) {
		if ip != "8.8.8.8" || db != "MaxMind GeoLite2" {
			t.Fatalf("unexpected geolocation lookup ip=%q db=%q", ip, db)
		}
		return &geolocation.Geolocation{
			Locale:    geolocation.Locale{Language: "en", Region: "US"},
			Longitude: -122.33,
			Latitude:  47.60,
			Timezone:  "America/Los_Angeles",
		}, nil
	}
	db := "MaxMind GeoLite2"
	built, err := BuildLaunchOptions(&LaunchOptions{
		Config: map[string]any{
			"timezone": "Asia/Jakarta",
		},
		GeoIP:            GeoIPAuto(),
		GeoIPDB:          &db,
		Proxy:            &ProxyConfig{Server: "socks5://127.0.0.1:1080", Username: "u", Password: "p"},
		ExecutablePath:   "camoufox",
		ExcludeAddons:    []DefaultAddon{AddonUBO},
		IKnowWhatImDoing: boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if seenProxy != "socks5://u:p@127.0.0.1:1080" {
		t.Fatalf("proxy was not used for public IP lookup: %q", seenProxy)
	}
	if built.Config["timezone"] != "Asia/Jakarta" {
		t.Fatalf("user timezone should win: %#v", built.Config)
	}
	if built.Config["locale:language"] != "en" || built.Config["locale:region"] != "US" {
		t.Fatalf("geo locale missing: %#v", built.Config)
	}
	if built.Config["geolocation:longitude"] != -122.33 || built.Config["geolocation:latitude"] != 47.60 {
		t.Fatalf("geo coordinates missing: %#v", built.Config)
	}
	if built.Config["webrtc:ipv4"] != "8.8.8.8" {
		t.Fatalf("webrtc ipv4 missing: %#v", built.Config)
	}
	if built.FirefoxUserPrefs["network.dns.disableIPv6"] != true {
		t.Fatalf("expected IPv6 DNS pref for IPv4 WebRTC spoofing: %#v", built.FirefoxUserPrefs)
	}
}

func TestBuildLaunchOptionsLocaleOverridesGeoIPLocale(t *testing.T) {
	oldGetGeolocation := getGeolocation
	defer func() { getGeolocation = oldGetGeolocation }()
	getGeolocation = func(ctx context.Context, ip, db string) (*geolocation.Geolocation, error) {
		return &geolocation.Geolocation{
			Locale:    geolocation.Locale{Language: "en", Region: "US"},
			Longitude: -122.33,
			Latitude:  47.60,
			Timezone:  "America/Los_Angeles",
		}, nil
	}
	built, err := BuildLaunchOptions(&LaunchOptions{
		GeoIP:            GeoIPFromIP("8.8.8.8"),
		Locale:           []string{"fr-FR"},
		ExecutablePath:   "camoufox",
		ExcludeAddons:    []DefaultAddon{AddonUBO},
		IKnowWhatImDoing: boolPtr(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if built.Config["locale:language"] != "fr" || built.Config["locale:region"] != "FR" {
		t.Fatalf("explicit locale should override geoip locale: %#v", built.Config)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
