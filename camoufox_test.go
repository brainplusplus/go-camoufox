package camoufox

import "testing"

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
