package fingerprint

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	mathrand "math/rand"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/brainplusplus/go-camoufox/internal/assets"
	"gopkg.in/yaml.v3"
)

const PresetsV150MinFirefox = 149

var (
	versionUARe  = regexp.MustCompile(`Firefox/\d+\.0`)
	versionRVRe  = regexp.MustCompile(`rv:\d+\.0`)
	versionAnyRe = regexp.MustCompile(`\b(1[0-9]{2})(\.0)\b`)

	globalRNGMu sync.Mutex
	globalRNG   = mathrand.New(mathrand.NewSource(cryptoSeed()))
)

type RNG interface {
	Intn(n int) int
	Float64() float64
	Uint32() uint32
}

type BrowserForgeFingerprint struct {
	Raw map[string]any
}

type ContextFingerprint struct {
	InitScript     string
	ContextOptions map[string]any
	Config         map[string]any
	Preset         map[string]any
}

type presetBundle struct {
	Presets map[string][]map[string]any `json:"presets"`
}

func LoadBrowserForgeMapping() (map[string]any, error) {
	data, err := assets.ReadFile("browserforge.yml")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := yaml.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func LoadPresets(ffVersion any) (map[string][]map[string]any, error) {
	file := "fingerprint-presets.json"
	if FirefoxMajor(ffVersion) >= PresetsV150MinFirefox {
		file = "fingerprint-presets-v150.json"
	}
	data, err := assets.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var bundle presetBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, err
	}
	return bundle.Presets, nil
}

func PresetCount(ffVersion any) (int, error) {
	presets, err := LoadPresets(ffVersion)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, values := range presets {
		total += len(values)
	}
	return total, nil
}

func GetRandomPreset(osValues []string, ffVersion any, rng RNG) (map[string]any, error) {
	presets, err := LoadPresets(ffVersion)
	if err != nil {
		return nil, err
	}
	keys := normalizeOSKeys(osValues)
	candidates := make([]map[string]any, 0)
	for _, key := range keys {
		candidates = append(candidates, presets[key]...)
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	return cloneMap(candidates[randIntn(rng, len(candidates))]), nil
}

func GenerateFingerprint(osValues []string, window *[2]int, ffVersion any, rng RNG) (*BrowserForgeFingerprint, error) {
	preset, err := GetRandomPreset(osValues, ffVersion, rng)
	if err != nil {
		return nil, err
	}
	if preset == nil {
		return nil, fmt.Errorf("no fingerprint presets available")
	}
	raw := cloneMap(preset)
	if window != nil {
		screen := mapValue(raw, "screen")
		screen["outerWidth"] = window[0]
		screen["outerHeight"] = window[1]
		if width, ok := intValue(screen["width"]); ok {
			screen["screenX"] = (width - window[0]) / 2
		}
		if height, ok := intValue(screen["height"]); ok {
			screen["screenY"] = (height - window[1]) / 2
		}
	}
	return &BrowserForgeFingerprint{Raw: raw}, nil
}

func FromBrowserForge(fp *BrowserForgeFingerprint, ffVersion any) (map[string]any, error) {
	if fp == nil || fp.Raw == nil {
		return nil, fmt.Errorf("fingerprint is empty")
	}
	mapping, err := LoadBrowserForgeMapping()
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	castToProperties(out, mapping, fp.Raw, majorString(ffVersion))
	if screen, ok := fp.Raw["screen"].(map[string]any); ok {
		handleScreenXY(out, screen)
	}
	return out, nil
}

func FromPreset(preset map[string]any, ffVersion any, rng RNG) (map[string]any, error) {
	if preset == nil {
		return nil, fmt.Errorf("preset is empty")
	}
	config := map[string]any{}
	nav := mapValue(preset, "navigator")
	if ua, ok := stringValue(nav["userAgent"]); ok {
		if major := majorString(ffVersion); major != "" {
			ua = versionUARe.ReplaceAllString(ua, "Firefox/"+major+".0")
			ua = versionRVRe.ReplaceAllString(ua, "rv:"+major+".0")
		}
		config["navigator.userAgent"] = ua
	}
	copyIfPresent(config, "navigator.platform", nav, "platform")
	copyIfPresent(config, "navigator.hardwareConcurrency", nav, "hardwareConcurrency")
	if _, ok := nav["oscpu"]; ok {
		copyIfPresent(config, "navigator.oscpu", nav, "oscpu")
	} else if platform, ok := stringValue(nav["platform"]); ok {
		config["navigator.oscpu"] = oscpuForPlatform(platform)
	}
	if value, ok := nav["maxTouchPoints"]; ok {
		config["navigator.maxTouchPoints"] = value
	}

	screen := mapValue(preset, "screen")
	copyIfPresent(config, "screen.width", screen, "width")
	copyIfPresent(config, "screen.height", screen, "height")
	if _, ok := screen["colorDepth"]; ok {
		copyIfPresent(config, "screen.colorDepth", screen, "colorDepth")
		config["screen.pixelDepth"] = screen["colorDepth"]
	}
	copyIfPresent(config, "screen.availWidth", screen, "availWidth")
	copyIfPresent(config, "screen.availHeight", screen, "availHeight")

	webgl := mapValue(preset, "webgl")
	copyIfPresent(config, "webGl:vendor", webgl, "unmaskedVendor")
	copyIfPresent(config, "webGl:renderer", webgl, "unmaskedRenderer")

	config["fonts:spacing_seed"] = randomSeed(rng)
	config["audio:seed"] = randomSeed(rng)
	config["canvas:seed"] = randomSeed(rng)
	copyIfPresent(config, "timezone", preset, "timezone")

	targetOS := osFromNavigator(nav)
	fonts, err := GenerateRandomFontSubset(targetOS, rng)
	if err == nil {
		config["fonts"] = fonts
	} else if fallback := stringSlice(preset["fonts"]); len(fallback) > 0 {
		EnsureMarkerFonts(&fallback, MarkerFonts(targetOS))
		config["fonts"] = fallback
	}
	voices, err := GenerateRandomVoiceSubset(targetOS, rng)
	if err == nil {
		config["voices"] = voices
	} else if fallback := stringSlice(preset["speechVoices"]); len(fallback) > 0 {
		config["voices"] = voiceNames(fallback)
	}
	return config, nil
}

func GenerateContextFingerprint(preset map[string]any, osValues []string, ffVersion any, webrtcIP, timezone, locale string, overrides map[string]any, rng RNG) (*ContextFingerprint, error) {
	if preset == nil {
		var err error
		preset, err = GetRandomPreset(osValues, ffVersion, rng)
		if err != nil {
			return nil, err
		}
	}
	config, err := FromPreset(preset, ffVersion, rng)
	if err != nil {
		return nil, err
	}
	if timezone != "" {
		config["timezone"] = timezone
	}
	if locale != "" {
		parts := strings.Split(locale, "-")
		config["locale:language"] = parts[0]
		if len(parts) > 1 {
			config["locale:region"] = parts[len(parts)-1]
		}
		config["navigator.language"] = locale
	}
	for key, value := range overrides {
		config[key] = value
	}
	initScript := BuildInitScript(map[string]any{
		"fontSpacingSeed":      config["fonts:spacing_seed"],
		"audioFingerprintSeed": config["audio:seed"],
		"canvasSeed":           config["canvas:seed"],
		"navigatorPlatform":    config["navigator.platform"],
		"navigatorOscpu":       config["navigator.oscpu"],
		"navigatorUserAgent":   config["navigator.userAgent"],
		"hardwareConcurrency":  config["navigator.hardwareConcurrency"],
		"webglVendor":          config["webGl:vendor"],
		"webglRenderer":        config["webGl:renderer"],
		"screenWidth":          config["screen.width"],
		"screenHeight":         config["screen.height"],
		"screenColorDepth":     config["screen.colorDepth"],
		"timezone":             config["timezone"],
		"fontList":             config["fonts"],
		"speechVoices":         config["voices"],
		"webrtcIP":             webrtcIP,
	})
	context := map[string]any{}
	if ua, ok := config["navigator.userAgent"]; ok {
		context["user_agent"] = ua
	}
	if width, wok := intValue(config["screen.width"]); wok {
		if height, hok := intValue(config["screen.height"]); hok {
			context["viewport"] = map[string]any{"width": width, "height": max(height-28, 600)}
		}
	}
	if tz, ok := stringValue(config["timezone"]); ok {
		context["timezone_id"] = tz
	}
	if lang, ok := stringValue(config["navigator.language"]); ok {
		context["locale"] = lang
	}
	return &ContextFingerprint{InitScript: initScript, ContextOptions: context, Config: config, Preset: cloneMap(preset)}, nil
}

func GenerateRandomFontSubset(targetOS string, rng RNG) ([]string, error) {
	var raw map[string][]string
	if err := readJSONAsset("fonts.json", &raw); err != nil {
		return nil, err
	}
	full := raw[assetOSKey(targetOS)]
	if len(full) == 0 {
		full = raw["mac"]
	}
	essential := set(EssentialFonts(targetOS))
	result := make([]string, 0, len(full))
	nonEssential := make([]string, 0, len(full))
	for _, font := range full {
		if _, ok := essential[font]; ok {
			result = append(result, font)
		} else {
			nonEssential = append(nonEssential, font)
		}
	}
	pct := 30 + int(randFloat64(rng)*49)
	count := int(math.Round((float64(pct) / 100) * float64(len(nonEssential))))
	result = append(result, sampleStrings(nonEssential, count, rng)...)
	EnsureMarkerFonts(&result, MarkerFonts(targetOS))
	return result, nil
}

func GenerateRandomVoiceSubset(targetOS string, rng RNG) ([]string, error) {
	var raw map[string][]string
	if err := readJSONAsset("voices.json", &raw); err != nil {
		return nil, err
	}
	full := voiceNames(raw[assetOSKey(targetOS)])
	if len(full) == 0 {
		return []string{}, nil
	}
	if targetOS == "windows" {
		return full, nil
	}
	essential := set(essentialVoices(targetOS))
	result := make([]string, 0, len(full))
	nonEssential := make([]string, 0, len(full))
	for _, voice := range full {
		if _, ok := essential[voice]; ok {
			result = append(result, voice)
		} else {
			nonEssential = append(nonEssential, voice)
		}
	}
	pct := 40 + int(randFloat64(rng)*41)
	count := int(math.Round((float64(pct) / 100) * float64(len(nonEssential))))
	result = append(result, sampleStrings(nonEssential, count, rng)...)
	return result, nil
}

func BuildInitScript(values map[string]any) string {
	var b strings.Builder
	b.WriteString("(function(v) {\n  var w = window;")
	setters := []struct{ key, fn string }{
		{"fontSpacingSeed", "setFontSpacingSeed"},
		{"audioFingerprintSeed", "setAudioFingerprintSeed"},
		{"canvasSeed", "setCanvasSeed"},
		{"navigatorPlatform", "setNavigatorPlatform"},
		{"navigatorOscpu", "setNavigatorOscpu"},
		{"navigatorUserAgent", "setNavigatorUserAgent"},
		{"hardwareConcurrency", "setNavigatorHardwareConcurrency"},
		{"webglVendor", "setWebGLVendor"},
		{"webglRenderer", "setWebGLRenderer"},
	}
	for _, setter := range setters {
		if value := values[setter.key]; value != nil {
			b.WriteString("\n  if (typeof w.")
			b.WriteString(setter.fn)
			b.WriteString(" === \"function\") w.")
			b.WriteString(setter.fn)
			b.WriteByte('(')
			b.WriteString(mustJSON(value))
			b.WriteString(");")
		}
	}
	if sw, ok := intValue(values["screenWidth"]); ok {
		if sh, ok := intValue(values["screenHeight"]); ok {
			b.WriteString(fmt.Sprintf("\n  if (typeof w.setScreenDimensions === \"function\") w.setScreenDimensions(%d, %d);", sw, sh))
			if scd, ok := intValue(values["screenColorDepth"]); ok {
				b.WriteString(fmt.Sprintf("\n  if (typeof w.setScreenColorDepth === \"function\") w.setScreenColorDepth(%d);", scd))
			}
		}
	}
	if tz, ok := stringValue(values["timezone"]); ok && tz != "" {
		b.WriteString("\n  if (typeof w.setTimezone === \"function\") w.setTimezone(")
		b.WriteString(mustJSON(tz))
		b.WriteString(");")
	}
	if ip, ok := stringValue(values["webrtcIP"]); ok && ip != "" {
		b.WriteString("\n  if (typeof w.setWebRTCIPv4 === \"function\") w.setWebRTCIPv4(")
		b.WriteString(mustJSON(ip))
		b.WriteString(");")
	} else {
		b.WriteString("\n  if (typeof w.setWebRTCIPv4 === \"function\") w.setWebRTCIPv4(\"\");")
	}
	if fonts := stringSlice(values["fontList"]); len(fonts) > 0 {
		b.WriteString("\n  if (typeof w.setFontList === \"function\") w.setFontList(")
		b.WriteString(mustJSON(strings.Join(fonts, ",")))
		b.WriteString(");")
	}
	if voices := stringSlice(values["speechVoices"]); len(voices) > 0 {
		b.WriteString("\n  if (typeof w.setSpeechVoices === \"function\") w.setSpeechVoices(")
		b.WriteString(mustJSON(strings.Join(voices, ",")))
		b.WriteString(");")
	}
	b.WriteString("\n})();")
	return b.String()
}

func SampleWebGL(targetOS, vendor, renderer string, rng RNG) (map[string]any, error) {
	presets, err := LoadPresets(PresetsV150MinFirefox)
	if err != nil {
		return nil, err
	}
	key := presetOSKey(targetOS)
	candidates := make([]map[string]any, 0)
	for _, preset := range presets[key] {
		webgl := mapValue(preset, "webgl")
		v, _ := stringValue(webgl["unmaskedVendor"])
		r, _ := stringValue(webgl["unmaskedRenderer"])
		if vendor != "" && renderer != "" && (v != vendor || r != renderer) {
			continue
		}
		if v != "" && r != "" {
			candidates = append(candidates, map[string]any{
				"webGl:vendor":   v,
				"webGl:renderer": r,
				"webGl2Enabled":  true,
			})
		}
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no WebGL data found for %s vendor %q renderer %q", targetOS, vendor, renderer)
	}
	return cloneMap(candidates[randIntn(rng, len(candidates))]), nil
}

func MarkerFonts(targetOS string) []string {
	switch targetOS {
	case "windows":
		return []string{"Segoe UI", "Tahoma", "Cambria Math", "Nirmala UI"}
	case "linux":
		return []string{"Arimo", "Cousine", "Tinos", "Twemoji Mozilla"}
	default:
		return []string{"Helvetica Neue", "PingFang HK", "PingFang SC", "PingFang TC"}
	}
}

func EssentialFonts(targetOS string) []string {
	switch targetOS {
	case "windows":
		return []string{"Arial", "Times New Roman", "Courier New", "Verdana", "Georgia", "Trebuchet MS", "Tahoma", "Segoe UI", "Calibri", "Cambria Math", "Nirmala UI", "Consolas"}
	case "linux":
		return []string{"Arimo", "Cousine", "Tinos", "Twemoji Mozilla", "Noto Sans Devanagari", "Noto Sans JP", "Noto Sans KR", "Noto Sans SC", "Noto Sans TC"}
	default:
		return []string{"Arial", "Helvetica", "Times New Roman", "Courier New", "Verdana", "Georgia", "Trebuchet MS", "Tahoma", "Helvetica Neue", "Lucida Grande", "Menlo", "Monaco", "Geneva", "PingFang HK", "PingFang SC", "PingFang TC"}
	}
}

func EnsureMarkerFonts(fonts *[]string, markers []string) {
	seen := set(*fonts)
	for _, marker := range markers {
		if _, ok := seen[marker]; !ok {
			*fonts = append(*fonts, marker)
			seen[marker] = struct{}{}
		}
	}
}

func FirefoxMajor(value any) int {
	switch typed := value.(type) {
	case nil:
		return 0
	case int:
		return typed
	case string:
		before, _, _ := strings.Cut(typed, ".")
		var n int
		_, _ = fmt.Sscanf(before, "%d", &n)
		return n
	default:
		return FirefoxMajor(fmt.Sprint(typed))
	}
}

func MergeInto(target, source map[string]any) {
	for key, value := range source {
		if _, ok := target[key]; !ok {
			target[key] = value
		}
	}
}

func SetInto(target map[string]any, key string, value any) {
	if _, ok := target[key]; !ok {
		target[key] = value
	}
}

func WithGlobalRNG(fn func(RNG) error) error {
	globalRNGMu.Lock()
	defer globalRNGMu.Unlock()
	return fn(globalRNG)
}

func readJSONAsset(name string, target any) error {
	data, err := assets.ReadFile(name)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func castToProperties(out map[string]any, mapping map[string]any, input map[string]any, ffVersion string) {
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := input[key]
		if isZero(value) {
			continue
		}
		typeKey, ok := mapping[key]
		if !ok {
			continue
		}
		if child, ok := value.(map[string]any); ok {
			if childMapping, ok := typeKey.(map[string]any); ok {
				castToProperties(out, childMapping, child, ffVersion)
			}
			continue
		}
		prop, ok := typeKey.(string)
		if !ok || prop == "" {
			continue
		}
		if strings.HasPrefix(prop, "screen.") {
			if number, ok := numberValue(value); ok && number < 0 {
				value = 0
			}
		}
		if ffVersion != "" {
			if text, ok := value.(string); ok {
				value = versionAnyRe.ReplaceAllString(text, ffVersion+"${2}")
			}
		}
		out[prop] = value
	}
}

func handleScreenXY(out map[string]any, screen map[string]any) {
	if _, ok := out["window.screenY"]; ok {
		return
	}
	screenX, ok := intValue(screen["screenX"])
	if !ok || screenX == 0 {
		out["window.screenX"] = 0
		out["window.screenY"] = 0
		return
	}
	if screenX >= -50 && screenX <= 50 {
		out["window.screenY"] = screenX
		return
	}
	availHeight, _ := intValue(screen["availHeight"])
	outerHeight, _ := intValue(screen["outerHeight"])
	delta := availHeight - outerHeight
	if delta == 0 {
		out["window.screenY"] = 0
	} else if delta > 0 {
		out["window.screenY"] = randIntn(nil, delta)
	} else {
		out["window.screenY"] = -randIntn(nil, -delta)
	}
}

func normalizeOSKeys(values []string) []string {
	if len(values) == 0 {
		return []string{"macos", "windows", "linux"}
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, presetOSKey(value))
	}
	return out
}

func presetOSKey(value string) string {
	switch value {
	case "win", "windows":
		return "windows"
	case "lin", "linux":
		return "linux"
	default:
		return "macos"
	}
}

func assetOSKey(value string) string {
	switch value {
	case "windows", "win":
		return "win"
	case "linux", "lin":
		return "lin"
	default:
		return "mac"
	}
}

func osFromNavigator(nav map[string]any) string {
	platform, _ := stringValue(nav["platform"])
	if platform == "Win32" {
		return "windows"
	}
	if strings.Contains(strings.ToLower(platform), "linux") {
		return "linux"
	}
	return "macos"
}

func oscpuForPlatform(platform string) string {
	switch {
	case platform == "MacIntel":
		return "Intel Mac OS X 10.15"
	case platform == "Win32":
		return "Windows NT 10.0; Win64; x64"
	case strings.Contains(strings.ToLower(platform), "linux"):
		return "Linux x86_64"
	default:
		return ""
	}
}

func targetOSFromConfig(config map[string]any) string {
	if platform, ok := stringValue(config["navigator.platform"]); ok {
		return osFromNavigator(map[string]any{"platform": platform})
	}
	if ua, ok := stringValue(config["navigator.userAgent"]); ok {
		switch {
		case strings.Contains(ua, "Windows"):
			return "windows"
		case strings.Contains(ua, "Linux"):
			return "linux"
		}
	}
	return "macos"
}

func majorString(value any) string {
	major := FirefoxMajor(value)
	if major <= 0 {
		return ""
	}
	return fmt.Sprint(major)
}

func randomSeed(rng RNG) uint32 {
	for {
		seed := randUint32(rng)
		if seed != 0 {
			return seed
		}
	}
}

func randIntn(rng RNG, n int) int {
	if rng != nil {
		return rng.Intn(n)
	}
	globalRNGMu.Lock()
	defer globalRNGMu.Unlock()
	return globalRNG.Intn(n)
}

func randFloat64(rng RNG) float64 {
	if rng != nil {
		return rng.Float64()
	}
	globalRNGMu.Lock()
	defer globalRNGMu.Unlock()
	return globalRNG.Float64()
}

func randUint32(rng RNG) uint32 {
	if rng != nil {
		return rng.Uint32()
	}
	globalRNGMu.Lock()
	defer globalRNGMu.Unlock()
	return globalRNG.Uint32()
}

func sampleStrings(values []string, count int, rng RNG) []string {
	if count >= len(values) {
		return append([]string(nil), values...)
	}
	if count <= 0 {
		return []string{}
	}
	perm := make([]int, len(values))
	for i := range perm {
		perm[i] = i
	}
	for i := len(perm) - 1; i > 0; i-- {
		j := randIntn(rng, i+1)
		perm[i], perm[j] = perm[j], perm[i]
	}
	out := make([]string, 0, count)
	for _, idx := range perm[:count] {
		out = append(out, values[idx])
	}
	return out
}

func essentialVoices(targetOS string) []string {
	if targetOS == "windows" {
		return []string{"Microsoft David - English (United States)", "Microsoft Zira - English (United States)", "Microsoft Mark - English (United States)"}
	}
	if targetOS == "macos" {
		return []string{"Samantha", "Alex", "Fred", "Victoria", "Karen", "Daniel"}
	}
	return nil
}

func voiceNames(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		name, _, _ := strings.Cut(value, ":")
		if name != "" {
			out = append(out, name)
		}
	}
	return out
}

func mapValue(input map[string]any, key string) map[string]any {
	if value, ok := input[key].(map[string]any); ok {
		return value
	}
	return map[string]any{}
}

func copyIfPresent(out map[string]any, outKey string, input map[string]any, inKey string) {
	if value, ok := input[inKey]; ok && !isZero(value) {
		out[outKey] = value
	}
}

func stringValue(value any) (string, bool) {
	text, ok := value.(string)
	return text, ok && text != ""
}

func intValue(value any) (int, bool) {
	number, ok := numberValue(value)
	return int(number), ok
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}

func stringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func set(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		case []any:
			out[key] = append([]any(nil), typed...)
		default:
			out[key] = typed
		}
	}
	return out
}

func isZero(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case int:
		return typed == 0
	case float64:
		return typed == 0
	case bool:
		return false
	default:
		return false
	}
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(data)
}

func cryptoSeed() int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 1
	}
	return int64(binary.LittleEndian.Uint64(buf[:]))
}

func TargetOSFromConfig(config map[string]any) string {
	return targetOSFromConfig(config)
}
