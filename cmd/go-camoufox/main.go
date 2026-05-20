package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	camoufox "github.com/brainplusplus/go-camoufox"
	"github.com/brainplusplus/go-camoufox/addons"
	"github.com/brainplusplus/go-camoufox/pkgman"
)

const version = "0.1.0-draft"

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
		return listInstalled()
	case "fetch":
		return fetch(args[1:])
	case "run":
		return runBrowser(args[1:])
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

func listInstalled() error {
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
		fmt.Printf("%s %s %s\n", repo, item.Version.FullString(), item.Path)
	}
	return nil
}

func printUsage() {
	fmt.Println("usage: go-camoufox <fetch|info|install-driver|list|path|run|version>")
}
