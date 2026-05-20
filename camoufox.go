package camoufox

import (
	"context"
	"errors"
	"os"

	"github.com/brainplusplus/go-camoufox/protocol/bidi"
	playwright "github.com/playwright-community/playwright-go"
)

type Browser struct {
	Options    *BuiltLaunchOptions
	Playwright *playwright.Playwright
	Browser    playwright.Browser
	Context    playwright.BrowserContext
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
	launched, err := pw.Firefox.Launch(toPlaywrightLaunchOptions(built))
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
	context, err := pw.Firefox.LaunchPersistentContext("", toPlaywrightPersistentOptions(built))
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

func toPlaywrightLaunchOptions(built *BuiltLaunchOptions) playwright.BrowserTypeLaunchOptions {
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
	return options
}

func toPlaywrightPersistentOptions(built *BuiltLaunchOptions) playwright.BrowserTypeLaunchPersistentContextOptions {
	options := playwright.BrowserTypeLaunchPersistentContextOptions{
		Args:             built.Args,
		Env:              built.Env,
		ExecutablePath:   &built.ExecutablePath,
		FirefoxUserPrefs: built.FirefoxUserPrefs,
		Headless:         &built.Headless,
	}
	if built.Proxy != nil {
		options.Proxy = toPlaywrightProxy(built.Proxy)
	}
	return options
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
