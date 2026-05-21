package camoufox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/brainplusplus/go-camoufox/fingerprint"
	"github.com/brainplusplus/go-camoufox/geolocation"
	"github.com/brainplusplus/go-camoufox/protocol/bidi"
	playwright "github.com/playwright-community/playwright-go"
)

type Browser struct {
	Options    *BuiltLaunchOptions
	Playwright *playwright.Playwright
	Browser    playwright.Browser
	Context    playwright.BrowserContext
}

type FingerprintedContext struct {
	Context     playwright.BrowserContext
	Fingerprint *fingerprint.ContextFingerprint
}

func New(ctx context.Context, opts *LaunchOptions) (*Browser, error) {
	return NewBrowser(ctx, nil, opts)
}

func NewBrowser(ctx context.Context, existing any, opts *LaunchOptions) (*Browser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	built, err := BuildLaunchOptions(opts)
	if err != nil {
		return nil, err
	}
	pw, owned, err := resolvePlaywright(existing)
	if err != nil {
		return nil, err
	}
	launchOptions, err := toPlaywrightLaunchOptions(built)
	if err != nil {
		return nil, err
	}
	launched, err := pw.Firefox.Launch(launchOptions)
	if err != nil {
		if built.VirtualDisplay != nil {
			_ = built.VirtualDisplay.Close()
		}
		if owned {
			_ = pw.Stop()
		}
		return nil, err
	}
	browser := &Browser{Options: built, Browser: launched}
	if owned {
		browser.Playwright = pw
	}
	return browser, nil
}

func NewContextFromBrowser(ctx context.Context, browser playwright.Browser, opts *ContextOptions) (*FingerprintedContext, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if browser == nil {
		return nil, errors.New("browser is required")
	}
	fp, contextOptions, err := buildContextOptions(ctx, opts, nil)
	if err != nil {
		return nil, err
	}
	context, err := browser.NewContext(contextOptions)
	if err != nil {
		return nil, err
	}
	if fp.InitScript != "" {
		script := fp.InitScript
		if err := context.AddInitScript(playwright.Script{Content: &script}); err != nil {
			_ = context.Close()
			return nil, err
		}
	}
	return &FingerprintedContext{Context: context, Fingerprint: fp}, nil
}

func (b *Browser) NewBrowserContext(ctx context.Context, opts *ContextOptions) (*FingerprintedContext, error) {
	if b == nil || b.Browser == nil {
		return nil, errors.New("browser handle is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fp, contextOptions, err := buildContextOptions(ctx, opts, b.Options)
	if err != nil {
		return nil, err
	}
	context, err := b.Browser.NewContext(contextOptions)
	if err != nil {
		return nil, err
	}
	if fp.InitScript != "" {
		script := fp.InitScript
		if err := context.AddInitScript(playwright.Script{Content: &script}); err != nil {
			_ = context.Close()
			return nil, err
		}
	}
	return &FingerprintedContext{Context: context, Fingerprint: fp}, nil
}

func NewContext(ctx context.Context, existing any, opts *LaunchOptions) (*Browser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	built, err := BuildLaunchOptions(opts)
	if err != nil {
		return nil, err
	}
	pw, owned, err := resolvePlaywright(existing)
	if err != nil {
		return nil, err
	}
	persistentOptions, err := toPlaywrightPersistentOptions(built)
	if err != nil {
		return nil, err
	}
	context, err := pw.Firefox.LaunchPersistentContext("", persistentOptions)
	if err != nil {
		if built.VirtualDisplay != nil {
			_ = built.VirtualDisplay.Close()
		}
		if owned {
			_ = pw.Stop()
		}
		return nil, err
	}
	browser := &Browser{Options: built, Context: context}
	if owned {
		browser.Playwright = pw
	}
	return browser, nil
}

func (b *Browser) Close(ctx context.Context) error {
	if b == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var closeErr error
	if b.Context != nil {
		closeErr = b.Context.Close()
	}
	if b.Browser != nil {
		if err := b.Browser.Close(); closeErr == nil {
			closeErr = err
		}
	}
	if b.Playwright != nil {
		if err := b.Playwright.Stop(); closeErr == nil {
			closeErr = err
		}
	}
	if b.Options != nil && b.Options.VirtualDisplay != nil {
		if err := b.Options.VirtualDisplay.Close(); closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func LaunchServer(ctx context.Context, opts *LaunchOptions) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	built, err := BuildLaunchOptions(opts)
	if err != nil {
		return "", err
	}
	server, err := LaunchServerHandle(ctx, built)
	if err != nil {
		return "", err
	}
	return server.Endpoint(), nil
}

func LaunchServerHandle(ctx context.Context, built *BuiltLaunchOptions) (*bidi.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if built == nil {
		return nil, errors.New("built launch options are required")
	}
	var proxy *bidi.ProxyConfig
	if built.Proxy != nil {
		proxy = &bidi.ProxyConfig{
			Server:   built.Proxy.Server,
			Bypass:   built.Proxy.Bypass,
			Username: built.Proxy.Username,
			Password: built.Proxy.Password,
		}
	}
	return bidi.Launch(ctx, bidi.Options{
		ExecutablePath: built.ExecutablePath,
		Args:           built.Args,
		Env:            built.Env,
		FirefoxPrefs:   built.FirefoxUserPrefs,
		Headless:       built.Headless,
		Proxy:          proxy,
		Stdout:         os.Stderr,
		Stderr:         os.Stderr,
	})
}

func LaunchServerHandleWithOptions(ctx context.Context, built *BuiltLaunchOptions, serverOptions bidi.Options) (*bidi.Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if built == nil {
		return nil, errors.New("built launch options are required")
	}
	var proxy *bidi.ProxyConfig
	if built.Proxy != nil {
		proxy = &bidi.ProxyConfig{
			Server:   built.Proxy.Server,
			Bypass:   built.Proxy.Bypass,
			Username: built.Proxy.Username,
			Password: built.Proxy.Password,
		}
	}
	serverOptions.ExecutablePath = built.ExecutablePath
	serverOptions.Args = built.Args
	serverOptions.Env = built.Env
	serverOptions.FirefoxPrefs = built.FirefoxUserPrefs
	serverOptions.Headless = built.Headless
	serverOptions.Proxy = proxy
	if serverOptions.Stdout == nil {
		serverOptions.Stdout = os.Stderr
	}
	if serverOptions.Stderr == nil {
		serverOptions.Stderr = os.Stderr
	}
	return bidi.Launch(ctx, serverOptions)
}

func resolvePlaywright(existing any) (*playwright.Playwright, bool, error) {
	if existing != nil {
		pw, ok := existing.(*playwright.Playwright)
		if !ok {
			return nil, false, errors.New("playwright handle must be *playwright.Playwright")
		}
		return pw, false, nil
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, false, err
	}
	return pw, true, nil
}

func InstallPlaywrightDriver(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return playwright.Install(&playwright.RunOptions{SkipInstallBrowsers: true})
}

func buildContextOptions(ctx context.Context, opts *ContextOptions, built *BuiltLaunchOptions) (*fingerprint.ContextFingerprint, playwright.BrowserNewContextOptions, error) {
	if opts == nil {
		opts = &ContextOptions{}
	}
	webrtcIP := opts.WebRTCIP
	timezone := opts.Timezone
	if opts.Proxy != nil && (webrtcIP == "" || timezone == "") {
		proxyString, err := (geolocation.Proxy{
			Server:   opts.Proxy.Server,
			Username: opts.Proxy.Username,
			Password: opts.Proxy.Password,
			Bypass:   opts.Proxy.Bypass,
		}).AsString()
		if err != nil {
			return nil, playwright.BrowserNewContextOptions{}, err
		}
		ip, err := publicIP(ctx, proxyString)
		if err == nil {
			if webrtcIP == "" {
				webrtcIP = ip
			}
			if timezone == "" {
				if geo, geoErr := getGeolocation(ctx, ip, ""); geoErr == nil {
					timezone = geo.Timezone
				}
			}
		}
	}
	ffVersion := defaultFirefoxMajor
	if opts.FFVersion != nil {
		ffVersion = *opts.FFVersion
	} else if built != nil {
		ffVersion = firefoxMajorFromConfig(built.Config, defaultFirefoxMajor)
	}
	fp, err := fingerprint.GenerateContextFingerprint(
		opts.Preset,
		opts.OS,
		strconv.Itoa(ffVersion),
		webrtcIP,
		timezone,
		opts.Locale,
		opts.Config,
		nil,
	)
	if err != nil {
		return nil, playwright.BrowserNewContextOptions{}, err
	}
	contextOptions := browserNewContextOptionsFromFingerprint(fp)
	mergeContextOptions(&contextOptions, opts)
	return fp, contextOptions, nil
}

func browserNewContextOptionsFromFingerprint(fp *fingerprint.ContextFingerprint) playwright.BrowserNewContextOptions {
	options := playwright.BrowserNewContextOptions{}
	if fp == nil {
		return options
	}
	if ua, ok := fp.ContextOptions["user_agent"].(string); ok && ua != "" {
		options.UserAgent = &ua
	}
	if viewport, ok := fp.ContextOptions["viewport"].(map[string]any); ok {
		if width, wok := configInt(viewport["width"]); wok {
			if height, hok := configInt(viewport["height"]); hok {
				options.Viewport = &playwright.Size{Width: width, Height: height}
			}
		}
	}
	if dpr, ok := numberOption(fp.ContextOptions["device_scale_factor"]); ok {
		options.DeviceScaleFactor = &dpr
	}
	if timezone, ok := fp.ContextOptions["timezone_id"].(string); ok && timezone != "" {
		options.TimezoneId = &timezone
	}
	if locale, ok := fp.ContextOptions["locale"].(string); ok && locale != "" {
		options.Locale = &locale
	}
	if width, height, ok := configScreenSize(fp.Config); ok {
		options.Screen = &playwright.Size{Width: width, Height: height}
	}
	return options
}

func mergeContextOptions(options *playwright.BrowserNewContextOptions, opts *ContextOptions) {
	if opts == nil {
		return
	}
	if opts.Proxy != nil {
		options.Proxy = toPlaywrightProxy(opts.Proxy)
	}
	if opts.Geolocation != nil {
		options.Geolocation = &playwright.Geolocation{
			Latitude:  opts.Geolocation.Latitude,
			Longitude: opts.Geolocation.Longitude,
		}
		if opts.Geolocation.Accuracy != nil {
			options.Geolocation.Accuracy = opts.Geolocation.Accuracy
		}
		options.Permissions = appendUnique(options.Permissions, "geolocation")
	}
	options.Permissions = appendUnique(options.Permissions, opts.Permissions...)
	if opts.Playwright == nil {
		return
	}
	if opts.Playwright.ExtraHTTPHeaders != nil {
		options.ExtraHttpHeaders = opts.Playwright.ExtraHTTPHeaders
	}
	if opts.Playwright.IgnoreHTTPSErrors != nil {
		options.IgnoreHttpsErrors = opts.Playwright.IgnoreHTTPSErrors
	}
	if opts.Playwright.JavaScriptEnabled != nil {
		options.JavaScriptEnabled = opts.Playwright.JavaScriptEnabled
	}
	if opts.Playwright.Offline != nil {
		options.Offline = opts.Playwright.Offline
	}
	if opts.Playwright.StrictSelectors != nil {
		options.StrictSelectors = opts.Playwright.StrictSelectors
	}
	if opts.Playwright.NoViewport != nil {
		options.NoViewport = opts.Playwright.NoViewport
	}
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]struct{}, len(values)+len(additions))
	out := make([]string, 0, len(values)+len(additions))
	for _, value := range append(values, additions...) {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func numberOption(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func firefoxMajorFromConfig(config map[string]any, fallback int) int {
	ua, _ := config["navigator.userAgent"].(string)
	if ua == "" {
		return fallback
	}
	_, after, ok := strings.Cut(ua, "Firefox/")
	if !ok {
		return fallback
	}
	major, err := strconv.Atoi(strings.SplitN(after, ".", 2)[0])
	if err != nil || major <= 0 {
		return fallback
	}
	return major
}

func toPlaywrightLaunchOptions(built *BuiltLaunchOptions) (playwright.BrowserTypeLaunchOptions, error) {
	options := playwright.BrowserTypeLaunchOptions{
		Args:             built.Args,
		Env:              built.Env,
		ExecutablePath:   &built.ExecutablePath,
		FirefoxUserPrefs: built.FirefoxUserPrefs,
		Headless:         &built.Headless,
	}
	if built.Proxy != nil {
		options.Proxy = toPlaywrightProxy(built.Proxy)
	}
	if err := applyLaunchExtra(&options, built.Extra); err != nil {
		return playwright.BrowserTypeLaunchOptions{}, err
	}
	return options, nil
}

func toPlaywrightPersistentOptions(built *BuiltLaunchOptions) (playwright.BrowserTypeLaunchPersistentContextOptions, error) {
	options := playwright.BrowserTypeLaunchPersistentContextOptions{
		Args:             built.Args,
		Env:              built.Env,
		ExecutablePath:   &built.ExecutablePath,
		FirefoxUserPrefs: built.FirefoxUserPrefs,
		Headless:         &built.Headless,
	}
	if width, height, ok := configScreenSize(built.Config); ok {
		options.Screen = &playwright.Size{Width: width, Height: height}
		options.Viewport = &playwright.Size{Width: width, Height: max(height-28, 600)}
	}
	if ua, ok := built.Config["navigator.userAgent"].(string); ok && ua != "" {
		options.UserAgent = &ua
	}
	if timezone, ok := built.Config["timezone"].(string); ok && timezone != "" {
		options.TimezoneId = &timezone
	}
	if language, ok := built.Config["navigator.language"].(string); ok && language != "" {
		options.Locale = &language
	}
	if built.Proxy != nil {
		options.Proxy = toPlaywrightProxy(built.Proxy)
	}
	if err := applyPersistentExtra(&options, built.Extra); err != nil {
		return playwright.BrowserTypeLaunchPersistentContextOptions{}, err
	}
	return options, nil
}

func configScreenSize(config map[string]any) (int, int, bool) {
	width, widthOK := configInt(config["screen.width"])
	height, heightOK := configInt(config["screen.height"])
	if !widthOK || !heightOK || width <= 0 || height <= 0 {
		return 0, 0, false
	}
	return width, height, true
}

func configInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err == nil {
			return int(i), true
		}
		f, err := v.Float64()
		if err == nil {
			return int(f), true
		}
	}
	return 0, false
}

func toPlaywrightProxy(proxy *ProxyConfig) *playwright.Proxy {
	out := &playwright.Proxy{Server: proxy.Server}
	if proxy.Bypass != "" {
		out.Bypass = &proxy.Bypass
	}
	if proxy.Username != "" {
		out.Username = &proxy.Username
	}
	if proxy.Password != "" {
		out.Password = &proxy.Password
	}
	return out
}
