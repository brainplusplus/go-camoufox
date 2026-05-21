package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	camoufox "github.com/brainplusplus/go-camoufox"
	"github.com/brainplusplus/go-camoufox/addons"
	"github.com/brainplusplus/go-camoufox/pkgman"
	"github.com/brainplusplus/go-camoufox/protocol/bidi"
)

var version = "0.2.1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "go-camoufox:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "version":
		fmt.Println(version)
		return nil
	case "path":
		dir, err := pkgman.CacheDir()
		if err != nil {
			return err
		}
		fmt.Println(dir)
		return nil
	case "info":
		dir, err := pkgman.CacheDir()
		if err != nil {
			return err
		}
		fmt.Printf("go-camoufox %s\ncache: %s\n", version, dir)
		return nil
	case "list":
		return listCmd(args[1:])
	case "active":
		return active()
	case "set":
		return setVersion(args[1:])
	case "sync":
		return syncRepos()
	case "remove":
		return remove(args[1:])
	case "fetch":
		return fetch(args[1:])
	case "run":
		return runBrowser(args[1:])
	case "server":
		return runServer(args[1:])
	case "install-driver":
		return camoufox.InstallPlaywrightDriver(context.Background())
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func fetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	list := fs.Bool("list", false, "list compatible releases")
	replace := fs.Bool("replace", false, "replace an existing install")
	versionFlag := fs.String("version", "", "install a specific version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	if *list {
		versions, err := pkgman.ListAvailableVersions(ctx, pkgman.InstallOptions{IncludePrerelease: true})
		if err != nil {
			return err
		}
		for _, version := range versions {
			suffix := ""
			if version.IsPrerelease {
				suffix = " (prerelease)"
			}
			fmt.Printf("v%s%s\n", version.Version.FullString(), suffix)
		}
		return nil
	}
	spec := ""
	if *versionFlag != "" {
		spec = *versionFlag
	}
	if fs.NArg() > 0 {
		spec = fs.Arg(0)
	}
	progress := io.Writer(os.Stderr)
	installed, err := pkgman.Install(ctx, pkgman.InstallOptions{
		VersionSpec:    spec,
		Replace:        *replace,
		DownloadWriter: progress,
	})
	if err != nil {
		return err
	}
	var addonPaths []string
	if err := addons.AddDefaultAddons(&addonPaths, nil); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to install default addons: %v\n", err)
	}
	fmt.Printf("installed v%s at %s\n", installed.Version.FullString(), installed.Path)
	return nil
}

func syncRepos() error {
	cache, err := pkgman.SyncRepoCache(context.Background(), pkgman.InstallOptions{IncludePrerelease: true})
	if err != nil {
		return err
	}
	total := 0
	for _, repo := range cache.Repos {
		total += len(repo.Versions)
	}
	fmt.Printf("synced %d versions from %d repos\n", total, len(cache.Repos))
	return nil
}

func setVersion(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		display, fetched, err := pkgman.ActiveDisplay()
		if err != nil {
			return err
		}
		if fetched {
			fmt.Println(display)
		} else {
			fmt.Printf("%s (not fetched)\n", display)
		}
		return nil
	}
	spec := fs.Arg(0)
	parts := strings.Split(spec, "/")
	switch len(parts) {
	case 2:
		config, err := pkgman.SetChannel(spec)
		if err != nil {
			return err
		}
		fmt.Printf("channel: %s\n", config.Channel)
		if config.ActiveVersion == "" {
			fmt.Println("run 'go-camoufox fetch' to install latest")
		}
	case 3:
		config, err := pkgman.SetPinned(spec)
		if err != nil {
			return err
		}
		fmt.Printf("pinned: %s/%s\n", config.Channel, config.Pinned)
		if config.ActiveVersion == "" {
			fmt.Println("run 'go-camoufox fetch' to install")
		}
	default:
		if item, err := pkgman.SetActive(spec); err == nil {
			fmt.Printf("active: %s\n", pkgman.InstalledChannelPath(*item))
			return nil
		}
		return fmt.Errorf("expected repo/channel, repo/channel/version, or installed version specifier")
	}
	return nil
}

func active() error {
	display, fetched, err := pkgman.ActiveDisplay()
	if err != nil {
		return err
	}
	if fetched {
		fmt.Println(display)
	} else {
		fmt.Printf("%s (not fetched)\n", display)
	}
	return nil
}

func remove(args []string) error {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip confirmation prompts")
	fs.BoolVar(yes, "y", false, "skip confirmation prompts")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !*yes {
		return fmt.Errorf("remove requires --yes in non-interactive Go CLI")
	}
	if fs.NArg() == 0 {
		if err := pkgman.RemoveAll(); err != nil {
			return err
		}
		fmt.Println("removed browser cache")
		return nil
	}
	item, err := pkgman.RemoveInstalled(fs.Arg(0))
	if err != nil {
		return err
	}
	fmt.Printf("removed %s\n", pkgman.InstalledChannelPath(*item))
	return nil
}

func runBrowser(args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	headless := fs.Bool("headless", false, "run headless")
	executablePath := fs.String("executable-path", "", "path to Camoufox executable")
	browser := fs.String("browser", "", "installed browser specifier")
	url := fs.String("url", "about:blank", "URL to open")
	noDefaultAddons := fs.Bool("no-default-addons", false, "skip default addons")
	installDriver := fs.Bool("install-driver", false, "install Playwright driver before launching")
	keepOpen := fs.Duration("keep-open", 0, "keep browser open for a duration, e.g. 10s")
	if err := fs.Parse(args); err != nil {
		return err
	}
	ctx := context.Background()
	if *installDriver {
		if err := camoufox.InstallPlaywrightDriver(ctx); err != nil {
			return err
		}
	}
	mode := camoufox.HeadlessFalse
	if *headless {
		mode = camoufox.HeadlessTrue
	}
	opts := &camoufox.LaunchOptions{
		Headless:       &mode,
		ExecutablePath: *executablePath,
	}
	if *browser != "" {
		opts.Browser = browser
	}
	if *noDefaultAddons {
		opts.ExcludeAddons = []camoufox.DefaultAddon{camoufox.AddonUBO}
	}
	b, err := camoufox.New(ctx, opts)
	if err != nil {
		return err
	}
	defer b.Close(ctx)
	if b.Browser != nil && *url != "" {
		page, err := b.Browser.NewPage()
		if err != nil {
			return err
		}
		if _, err := page.Goto(*url); err != nil {
			return err
		}
	}
	if *keepOpen > 0 {
		time.Sleep(*keepOpen)
	}
	fmt.Println("browser launched successfully")
	return nil
}

func runServer(args []string) error {
	fs := flag.NewFlagSet("server", flag.ContinueOnError)
	headless := fs.Bool("headless", false, "run headless")
	executablePath := fs.String("executable-path", "", "path to Camoufox executable")
	browser := fs.String("browser", "", "installed browser specifier")
	noDefaultAddons := fs.Bool("no-default-addons", false, "skip default addons")
	optionsJSON := fs.String("options-json", "", "inline LaunchOptions JSON or path to a JSON file")
	proxyServer := fs.String("proxy-server", "", "proxy server URL")
	proxyBypass := fs.String("proxy-bypass", "", "proxy bypass list")
	proxyUsername := fs.String("proxy-username", "", "proxy username")
	proxyPassword := fs.String("proxy-password", "", "proxy password")
	virtualDisplay := fs.String("virtual-display", "", "DISPLAY value for virtual display mode")
	listen := fs.String("listen", "127.0.0.1:0", "BiDi server listen address")
	var osValues, locales, extraArgs, envValues, prefValues, configValues repeatFlag
	fs.Var(&osValues, "os", "fingerprint OS target; repeatable")
	fs.Var(&locales, "locale", "locale such as en-US; repeatable")
	fs.Var(&extraArgs, "arg", "extra browser argument; repeatable")
	fs.Var(&envValues, "env", "environment KEY=VALUE; repeatable")
	fs.Var(&prefValues, "pref", "Firefox pref KEY=VALUE; repeatable")
	fs.Var(&configValues, "config", "Camoufox config KEY=VALUE; repeatable")
	blockImages := fs.Bool("block-images", false, "block images")
	blockWebRTC := fs.Bool("block-webrtc", false, "block WebRTC")
	blockWebGL := fs.Bool("block-webgl", false, "block WebGL")
	enableCache := fs.Bool("enable-cache", false, "enable Firefox cache prefs")
	mainWorldEval := fs.Bool("main-world-eval", false, "allow main-world evaluation")
	iKnow := fs.Bool("i-know-what-im-doing", false, "suppress leak warnings")
	if err := fs.Parse(args); err != nil {
		return err
	}

	opts := &camoufox.LaunchOptions{}
	if *optionsJSON != "" {
		loaded, err := loadLaunchOptionsJSON(*optionsJSON)
		if err != nil {
			return err
		}
		opts = loaded
	}
	mode := camoufox.HeadlessFalse
	if *headless {
		mode = camoufox.HeadlessTrue
		opts.Headless = &mode
	}
	opts.ExecutablePath = firstNonEmpty(*executablePath, opts.ExecutablePath)
	if *browser != "" {
		opts.Browser = browser
	}
	if *noDefaultAddons {
		opts.ExcludeAddons = []camoufox.DefaultAddon{camoufox.AddonUBO}
	}
	if len(osValues) > 0 {
		opts.OS = append([]string(nil), osValues...)
	}
	if len(locales) > 0 {
		opts.Locale = append([]string(nil), locales...)
	}
	if len(extraArgs) > 0 {
		opts.Args = append(opts.Args, extraArgs...)
	}
	if *virtualDisplay != "" {
		opts.VirtualDisplay = virtualDisplay
	}
	if *proxyServer != "" {
		opts.Proxy = &camoufox.ProxyConfig{
			Server:   *proxyServer,
			Bypass:   *proxyBypass,
			Username: *proxyUsername,
			Password: *proxyPassword,
		}
	}
	if err := mergePairs(&opts.Env, envValues); err != nil {
		return err
	}
	if err := mergePairs(&opts.FirefoxUserPrefs, prefValues); err != nil {
		return err
	}
	if err := mergePairs(&opts.Config, configValues); err != nil {
		return err
	}
	applyBoolFlag(fs, "block-images", &opts.BlockImages, *blockImages)
	applyBoolFlag(fs, "block-webrtc", &opts.BlockWebRTC, *blockWebRTC)
	applyBoolFlag(fs, "block-webgl", &opts.BlockWebGL, *blockWebGL)
	applyBoolFlag(fs, "enable-cache", &opts.EnableCache, *enableCache)
	applyBoolFlag(fs, "main-world-eval", &opts.MainWorldEval, *mainWorldEval)
	applyBoolFlag(fs, "i-know-what-im-doing", &opts.IKnowWhatImDoing, *iKnow)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	built, err := camoufox.BuildLaunchOptions(opts)
	if err != nil {
		return err
	}
	server, err := camoufox.LaunchServerHandleWithOptions(ctx, built, bidiOptions(*listen))
	if err != nil {
		return err
	}
	fmt.Println(server.Endpoint())
	select {
	case <-ctx.Done():
		_ = server.Close()
	case <-server.Done():
	}
	return nil
}

func listCmd(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	showPaths := fs.Bool("path", false, "show full paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	mode := "installed"
	if fs.NArg() > 0 {
		mode = fs.Arg(0)
	}
	if mode == "all" {
		return listAll(*showPaths)
	}
	if mode != "installed" {
		return fmt.Errorf("list mode must be installed or all")
	}
	return listInstalled(*showPaths)
}

func listInstalled(showPaths bool) error {
	installed, err := pkgman.ListInstalled()
	if err != nil {
		return err
	}
	if len(installed) == 0 {
		fmt.Println("no installed browsers")
		return nil
	}
	for _, item := range installed {
		repo := item.Repo
		if repo == "" {
			repo = "unknown"
		}
		active := ""
		if item.IsActive {
			active = " (active)"
		}
		if showPaths {
			fmt.Printf("%s/%s %s%s %s\n", strings.ToLower(repo), item.Channel, item.Version.FullString(), active, item.Path)
		} else {
			fmt.Printf("%s %s%s\n", strings.ToLower(repo), item.Version.FullString(), active)
		}
	}
	return nil
}

func listAll(showPaths bool) error {
	cache, err := pkgman.LoadRepoCache()
	if err != nil {
		return err
	}
	if len(cache.Repos) == 0 {
		return fmt.Errorf("no repo cache found; run go-camoufox sync first")
	}
	installed, _ := pkgman.ListInstalled()
	installedByBuild := map[string]pkgman.InstalledVersion{}
	for _, item := range installed {
		installedByBuild[item.Version.Build] = item
	}
	for _, repo := range cache.Repos {
		fmt.Printf("%s/\n", strings.ToLower(repo.Name))
		for _, version := range repo.Versions {
			full := version.Version + "-" + version.Build
			suffix := " (stable)"
			if version.IsPrerelease {
				suffix = " (prerelease)"
			}
			if inst, ok := installedByBuild[version.Build]; ok {
				if inst.IsActive {
					suffix += " (installed, active)"
				} else {
					suffix += " (installed)"
				}
				if showPaths {
					suffix += " " + inst.Path
				}
			}
			fmt.Printf("  v%s%s\n", full, suffix)
		}
	}
	return nil
}

func printUsage() {
	fmt.Println("usage: go-camoufox <active|fetch|info|install-driver|list|path|remove|run|server|set|sync|version>")
}

type repeatFlag []string

func (r *repeatFlag) String() string { return strings.Join(*r, ",") }

func (r *repeatFlag) Set(value string) error {
	*r = append(*r, value)
	return nil
}

func loadLaunchOptionsJSON(value string) (*camoufox.LaunchOptions, error) {
	data := []byte(value)
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		var readErr error
		data, readErr = os.ReadFile(value)
		if readErr != nil {
			return nil, readErr
		}
	}
	var opts camoufox.LaunchOptions
	if err := json.Unmarshal(data, &opts); err != nil {
		return nil, err
	}
	return &opts, nil
}

func mergePairs(dst *map[string]any, pairs []string) error {
	if len(pairs) == 0 {
		return nil
	}
	if *dst == nil {
		*dst = map[string]any{}
	}
	for _, pair := range pairs {
		key, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return fmt.Errorf("expected KEY=VALUE, got %q", pair)
		}
		(*dst)[key] = parseScalar(value)
	}
	return nil
}

func parseScalar(value string) any {
	if parsed, err := strconv.ParseBool(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	return value
}

func applyBoolFlag(fs *flag.FlagSet, name string, target **bool, value bool) {
	seen := false
	fs.Visit(func(flag *flag.Flag) {
		if flag.Name == name {
			seen = true
		}
	})
	if seen {
		*target = &value
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func bidiOptions(listen string) bidi.Options {
	return bidi.Options{ListenAddr: listen}
}
