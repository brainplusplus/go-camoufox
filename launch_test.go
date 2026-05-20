package camoufox

import (
	"encoding/json"
	"strings"
	"testing"
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

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
