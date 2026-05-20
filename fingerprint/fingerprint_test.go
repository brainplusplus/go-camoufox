package fingerprint

import (
	"reflect"
	"strings"
	"testing"
)

type sequenceRNG struct {
	ints   []int
	floats []float64
	uints  []uint32
}

func (r *sequenceRNG) Intn(n int) int {
	if len(r.ints) == 0 {
		return 0
	}
	value := r.ints[0] % n
	r.ints = r.ints[1:]
	return value
}

func (r *sequenceRNG) Float64() float64 {
	if len(r.floats) == 0 {
		return 0
	}
	value := r.floats[0]
	r.floats = r.floats[1:]
	return value
}

func (r *sequenceRNG) Uint32() uint32 {
	if len(r.uints) == 0 {
		return 1
	}
	value := r.uints[0]
	r.uints = r.uints[1:]
	return value
}

func TestPresetCountsAndVersionSwitch(t *testing.T) {
	original, err := PresetCount(148)
	if err != nil {
		t.Fatal(err)
	}
	if original != 123 {
		t.Fatalf("expected original preset count 123, got %d", original)
	}
	v150, err := PresetCount(149)
	if err != nil {
		t.Fatal(err)
	}
	if v150 != 312 {
		t.Fatalf("expected v150 preset count 312, got %d", v150)
	}
}

func TestRandomFontSubsetIncludesMarkerFonts(t *testing.T) {
	for osName, markers := range map[string][]string{
		"macos":   {"Helvetica Neue", "PingFang HK", "PingFang SC", "PingFang TC"},
		"linux":   {"Arimo", "Cousine", "Tinos", "Twemoji Mozilla"},
		"windows": {"Segoe UI", "Tahoma", "Cambria Math", "Nirmala UI"},
	} {
		fonts, err := GenerateRandomFontSubset(osName, &sequenceRNG{floats: []float64{0}})
		if err != nil {
			t.Fatal(err)
		}
		have := set(fonts)
		for _, marker := range markers {
			if _, ok := have[marker]; !ok {
				t.Fatalf("%s subset missing marker font %q", osName, marker)
			}
		}
	}
}

func TestRandomVoiceSubsetMatchesReferenceShape(t *testing.T) {
	linux, err := GenerateRandomVoiceSubset("linux", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(linux) != 0 {
		t.Fatalf("expected linux voices to be empty, got %d", len(linux))
	}
	windows, err := GenerateRandomVoiceSubset("windows", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(windows) != 53 {
		t.Fatalf("expected all windows voices, got %d", len(windows))
	}
	if strings.Contains(windows[0], ":") {
		t.Fatalf("voice should be stripped to name, got %q", windows[0])
	}
}

func TestFromPresetConvertsReferenceKeys(t *testing.T) {
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
	config, err := FromPreset(preset, "151", &sequenceRNG{uints: []uint32{11, 22, 33}, floats: []float64{0}})
	if err != nil {
		t.Fatal(err)
	}
	if config["navigator.userAgent"] != "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:151.0) Gecko/20100101 Firefox/151.0" {
		t.Fatalf("UA version replacement failed: %v", config["navigator.userAgent"])
	}
	if config["navigator.oscpu"] != "Windows NT 10.0; Win64; x64" {
		t.Fatalf("oscpu not derived: %v", config["navigator.oscpu"])
	}
	if _, ok := config["navigator.maxTouchPoints"]; !ok {
		t.Fatal("maxTouchPoints=0 must be preserved")
	}
	if config["fonts:spacing_seed"] != uint32(11) || config["audio:seed"] != uint32(22) || config["canvas:seed"] != uint32(33) {
		t.Fatalf("unexpected seeds: %#v", config)
	}
	if !reflect.DeepEqual(MarkerFonts("windows"), []string{"Segoe UI", "Tahoma", "Cambria Math", "Nirmala UI"}) {
		t.Fatal("windows marker font list changed")
	}
}

func TestBuildInitScriptParityFields(t *testing.T) {
	script := BuildInitScript(map[string]any{
		"fontSpacingSeed":      uint32(1),
		"audioFingerprintSeed": uint32(2),
		"canvasSeed":           uint32(3),
		"navigatorPlatform":    "Win32",
		"navigatorOscpu":       "Windows NT 10.0; Win64; x64",
		"navigatorUserAgent":   "Mozilla/5.0 Firefox/150.0",
		"webglVendor":          "Vendor",
		"webglRenderer":        "Renderer",
		"screenWidth":          1920,
		"screenHeight":         1080,
		"screenColorDepth":     24,
		"timezone":             "America/New_York",
		"fontList":             []string{"Arial", "Tahoma"},
		"speechVoices":         []string{"Microsoft David - English (United States)"},
	})
	for _, want := range []string{
		"setFontSpacingSeed(1)",
		"setNavigatorPlatform(\"Win32\")",
		"setWebGLVendor(\"Vendor\")",
		"setScreenDimensions(1920, 1080)",
		"setTimezone(\"America/New_York\")",
		"setWebRTCIPv4(\"\")",
		"setFontList(\"Arial,Tahoma\")",
		"setSpeechVoices(\"Microsoft David - English (United States)\")",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("init script missing %q:\n%s", want, script)
		}
	}
}
