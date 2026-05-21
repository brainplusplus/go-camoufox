package camoufox

import (
	"testing"

	"github.com/brainplusplus/go-camoufox/fingerprint"
	playwright "github.com/playwright-community/playwright-go"
)

func TestPlaywrightLaunchOptionMapping(t *testing.T) {
	built := &BuiltLaunchOptions{
		ExecutablePath:   "C:/camoufox/camoufox.exe",
		Args:             []string{"--foo"},
		Env:              map[string]string{"CAMOU_CONFIG_1": "{}"},
		FirefoxUserPrefs: map[string]any{"media.peerconnection.enabled": false},
		Headless:         true,
		Proxy: &ProxyConfig{
			Server:   "socks5://127.0.0.1:1080",
			Bypass:   "localhost",
			Username: "u",
			Password: "p",
		},
	}
	options := toPlaywrightLaunchOptions(built)
	if options.ExecutablePath == nil || *options.ExecutablePath != built.ExecutablePath {
		t.Fatal("executable path was not mapped")
	}
	if options.Headless == nil || !*options.Headless {
		t.Fatal("headless was not mapped")
	}
	if options.Proxy == nil || options.Proxy.Server != built.Proxy.Server {
		t.Fatal("proxy was not mapped")
	}
	if options.Proxy.Bypass == nil || *options.Proxy.Bypass != "localhost" {
		t.Fatal("proxy bypass was not mapped")
	}
	if options.FirefoxUserPrefs["media.peerconnection.enabled"] != false {
		t.Fatal("firefox prefs were not mapped")
	}
}

func TestContextFingerprintOptionsMapToPlaywright(t *testing.T) {
	preset := map[string]any{
		"navigator": map[string]any{
			"userAgent":           "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:150.0) Gecko/20100101 Firefox/150.0",
			"platform":            "Win32",
			"hardwareConcurrency": float64(12),
		},
		"screen": map[string]any{
			"width":            float64(1920),
			"height":           float64(1080),
			"colorDepth":       float64(24),
			"availWidth":       float64(1920),
			"availHeight":      float64(1032),
			"devicePixelRatio": float64(1.25),
		},
		"webgl": map[string]any{
			"unmaskedVendor":   "Google Inc. (Intel)",
			"unmaskedRenderer": "ANGLE (Intel, Intel(R) HD Graphics Direct3D11 vs_5_0 ps_5_0), or similar",
		},
	}
	fp, err := fingerprint.GenerateContextFingerprint(
		preset,
		[]string{"windows"},
		"151",
		"203.0.113.1",
		"Asia/Jakarta",
		"id-ID",
		map[string]any{"fonts:spacing_seed": uint32(9)},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	options := browserNewContextOptionsFromFingerprint(fp)
	if options.UserAgent == nil || *options.UserAgent != "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:151.0) Gecko/20100101 Firefox/151.0" {
		t.Fatalf("user agent option missing: %#v", options.UserAgent)
	}
	if options.Viewport == nil || options.Viewport.Width != 1920 || options.Viewport.Height != 1052 {
		t.Fatalf("viewport option missing: %#v", options.Viewport)
	}
	if options.Screen == nil || options.Screen.Width != 1920 || options.Screen.Height != 1080 {
		t.Fatalf("screen option missing: %#v", options.Screen)
	}
	if options.DeviceScaleFactor == nil || *options.DeviceScaleFactor != 1.25 {
		t.Fatalf("device scale factor missing: %#v", options.DeviceScaleFactor)
	}
	if options.TimezoneId == nil || *options.TimezoneId != "Asia/Jakarta" {
		t.Fatalf("timezone option missing: %#v", options.TimezoneId)
	}
	if options.Locale == nil || *options.Locale != "id-ID" {
		t.Fatalf("locale option missing: %#v", options.Locale)
	}
}

func TestMergeContextOptionsUserValuesWin(t *testing.T) {
	options := playwrightContextOptionsFixture()
	ignoreHTTPSErrors := true
	offline := true
	noViewport := true
	accuracy := 12.5
	mergeContextOptions(&options, &ContextOptions{
		Proxy:       &ProxyConfig{Server: "socks5://127.0.0.1:1080"},
		Geolocation: &GeolocationOption{Latitude: -6.2, Longitude: 106.8, Accuracy: &accuracy},
		Permissions: []string{"clipboard-read"},
		Playwright: &PlaywrightContextOptions{
			ExtraHTTPHeaders:  map[string]string{"x-test": "1"},
			IgnoreHTTPSErrors: &ignoreHTTPSErrors,
			Offline:           &offline,
			NoViewport:        &noViewport,
		},
	})
	if options.Proxy == nil || options.Proxy.Server != "socks5://127.0.0.1:1080" {
		t.Fatalf("proxy missing: %#v", options.Proxy)
	}
	if options.Geolocation == nil || options.Geolocation.Latitude != -6.2 || options.Geolocation.Longitude != 106.8 {
		t.Fatalf("geolocation missing: %#v", options.Geolocation)
	}
	if len(options.Permissions) != 2 || options.Permissions[0] != "geolocation" || options.Permissions[1] != "clipboard-read" {
		t.Fatalf("permissions not merged: %#v", options.Permissions)
	}
	if options.ExtraHttpHeaders["x-test"] != "1" || options.IgnoreHttpsErrors == nil || !*options.IgnoreHttpsErrors {
		t.Fatalf("playwright options missing: %#v", options)
	}
	if options.Offline == nil || !*options.Offline || options.NoViewport == nil || !*options.NoViewport {
		t.Fatalf("boolean options missing: %#v", options)
	}
}

func playwrightContextOptionsFixture() playwright.BrowserNewContextOptions {
	return playwright.BrowserNewContextOptions{}
}
