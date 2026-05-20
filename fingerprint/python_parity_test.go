package fingerprint

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPythonReferencePresetCountsParity(t *testing.T) {
	out := runPythonReference(t, "preset_counts", nil)
	var counts map[string]int
	if err := json.Unmarshal(out, &counts); err != nil {
		t.Fatal(err)
	}
	for version, want := range counts {
		got, err := PresetCount(version)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("preset count mismatch for Firefox %s: Go=%d Python=%d", version, got, want)
		}
	}
}

func TestPythonReferenceFromPresetParity(t *testing.T) {
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
		"timezone": "America/New_York",
	}
	input, err := json.Marshal(preset)
	if err != nil {
		t.Fatal(err)
	}
	pyOut := runPythonReference(t, "from_preset", input)
	var py map[string]any
	if err := json.Unmarshal(pyOut, &py); err != nil {
		t.Fatal(err)
	}
	goOut, err := FromPreset(preset, "151", &sequenceRNG{uints: []uint32{1, 2, 3}, floats: []float64{0}})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		"navigator.userAgent",
		"navigator.platform",
		"navigator.hardwareConcurrency",
		"navigator.oscpu",
		"navigator.maxTouchPoints",
		"screen.width",
		"screen.height",
		"screen.colorDepth",
		"screen.pixelDepth",
		"screen.availWidth",
		"screen.availHeight",
		"webGl:vendor",
		"webGl:renderer",
		"timezone",
	} {
		if !jsonEqual(goOut[key], py[key]) {
			t.Fatalf("from_preset mismatch for %s: Go=%#v Python=%#v", key, goOut[key], py[key])
		}
	}
}

func TestPythonReferenceInitScriptParity(t *testing.T) {
	values := map[string]any{
		"fontSpacingSeed":      float64(1),
		"audioFingerprintSeed": float64(2),
		"canvasSeed":           float64(3),
		"navigatorPlatform":    "Win32",
		"navigatorOscpu":       "Windows NT 10.0; Win64; x64",
		"navigatorUserAgent":   "Mozilla/5.0 Firefox/150.0",
		"hardwareConcurrency":  float64(12),
		"webglVendor":          "Vendor",
		"webglRenderer":        "Renderer",
		"screenWidth":          float64(1920),
		"screenHeight":         float64(1080),
		"screenColorDepth":     float64(24),
		"timezone":             "America/New_York",
		"webrtcIP":             "",
		"fontList":             []any{"Arial", "Tahoma"},
		"speechVoices":         []any{"Microsoft David - English (United States)"},
	}
	input, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	var pyScript string
	if err := json.Unmarshal(runPythonReference(t, "init_script", input), &pyScript); err != nil {
		t.Fatal(err)
	}
	goScript := BuildInitScript(values)
	if goScript != pyScript {
		t.Fatalf("init script mismatch:\nGo:\n%s\nPython:\n%s", goScript, pyScript)
	}
}

func runPythonReference(t *testing.T, command string, stdin []byte) []byte {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("python executable not available for reference parity harness")
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test path")
	}
	root := filepath.Dir(filepath.Dir(file))
	script := filepath.Join(root, "internal", "parity", "python_reference.py")
	cmd := exec.Command(python, script, command)
	cmd.Dir = root
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("python reference %s failed: %v\n%s", command, err, stderr.String())
	}
	return bytes.TrimSpace(out)
}

func jsonEqual(left, right any) bool {
	l, _ := json.Marshal(left)
	r, _ := json.Marshal(right)
	return bytes.Equal(l, r)
}
