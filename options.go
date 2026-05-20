package camoufox

import "github.com/brainplusplus/go-camoufox/internal/types"

type DefaultAddon = types.DefaultAddon

const AddonUBO = types.AddonUBO

type HeadlessMode string

const (
	HeadlessFalse   HeadlessMode = "false"
	HeadlessTrue    HeadlessMode = "true"
	HeadlessVirtual HeadlessMode = "virtual"
)

type GeoIPOption struct {
	Auto bool
	IP   string
}

func GeoIPAuto() *GeoIPOption {
	return &GeoIPOption{Auto: true}
}

func GeoIPFromIP(ip string) *GeoIPOption {
	return &GeoIPOption{IP: ip}
}

type HumanizeOption struct {
	Enabled bool
	MaxTime *float64
}

type ScreenConstraint struct {
	MinWidth  int
	MaxWidth  int
	MinHeight int
	MaxHeight int
}

type Fingerprint struct {
	UserAgent string
	Screen    *ScreenConstraint
	Raw       map[string]any
}

type FingerprintPreset struct {
	UseRandom bool
	Preset    map[string]any
}

type ProxyConfig struct {
	Server   string `json:"server,omitempty"`
	Bypass   string `json:"bypass,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type LaunchOptions struct {
	Config            map[string]any
	OS                []string
	BlockImages       *bool
	BlockWebRTC       *bool
	BlockWebGL        *bool
	DisableCOOP       *bool
	WebGLConfig       *[2]string
	GeoIP             *GeoIPOption
	GeoIPDB           *string
	Humanize          *HumanizeOption
	Locale            []string
	Addons            []string
	Fonts             []string
	CustomFontsOnly   *bool
	ExcludeAddons     []DefaultAddon
	Screen            *ScreenConstraint
	Window            *[2]int
	Fingerprint       *Fingerprint
	FingerprintPreset *FingerprintPreset
	FFVersion         *int
	Headless          *HeadlessMode
	MainWorldEval     *bool
	ExecutablePath    string
	Browser           *string
	FirefoxUserPrefs  map[string]any
	Proxy             *ProxyConfig
	EnableCache       *bool
	Args              []string
	Env               map[string]any
	IKnowWhatImDoing  *bool
	Debug             *bool
	VirtualDisplay    *string
	Extra             map[string]any
}

type BuiltLaunchOptions struct {
	ExecutablePath   string
	Args             []string
	Env              map[string]string
	FirefoxUserPrefs map[string]any
	Headless         bool
	Proxy            *ProxyConfig
	Extra            map[string]any
	Config           map[string]any
	VirtualDisplay   interface{ Close() error }
}
