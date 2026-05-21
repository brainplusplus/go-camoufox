package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	camoufox "github.com/brainplusplus/go-camoufox"
	playwright "github.com/playwright-community/playwright-go"
)

func main() {
	ctx := context.Background()
	autoCloseSeconds, _ := strconv.ParseFloat(os.Getenv("CAMOUFOX_AUTO_CLOSE_SECONDS"), 64)

	headless := camoufox.HeadlessFalse
	browser, err := camoufox.New(ctx, &camoufox.LaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close(ctx)

	page, err := browser.Browser.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := page.Goto("https://www.google.com"); err != nil {
		log.Fatal(err)
	}

	timeout := 30000.0
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: &timeout,
	}); err != nil {
		log.Printf("networkidle wait failed: %v", err)
	}

	title, err := page.Title()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Page title: %s\n", title)

	searchBox, err := page.QuerySelector("textarea[name='q'], input[name='q']")
	if err != nil {
		log.Fatal(err)
	}
	if searchBox != nil {
		label, err := searchBox.GetAttribute("aria-label")
		if err != nil {
			log.Fatal(err)
		}
		if label == "" {
			label = "(empty)"
		}
		fmt.Printf("Search box label: %s\n", label)

		query := "go-camoufox github"
		if err := searchBox.Click(); err != nil {
			log.Fatal(err)
		}
		if err := searchBox.Type(query, playwright.ElementHandleTypeOptions{
			Delay: playwright.Float(120),
		}); err != nil {
			log.Fatal(err)
		}
		if err := searchBox.Press("Enter"); err != nil {
			log.Fatal(err)
		}

		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateNetworkidle,
			Timeout: &timeout,
		}); err != nil {
			log.Printf("search results networkidle wait failed: %v", err)
		}

		resultsTitle, err := page.Title()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Results title: %s\n", resultsTitle)
		fmt.Printf("Current URL: %s\n", page.URL())

		challenge, err := page.Evaluate(`
			(() => {
			  const text = document.body ? document.body.innerText : "";
			  return /unusual traffic|not a robot|captcha/i.test(text);
			})()
		`)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Possible challenge detected: %v\n", challenge)
	} else {
		fmt.Println("Search box not found - page layout may differ in this region.")
	}

	ua, err := page.Evaluate("navigator.userAgent")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User-Agent: %v\n", ua)
	printWindowMetrics(page)

	if autoCloseSeconds > 0 {
		fmt.Printf("\nAuto-closing in %.1f seconds...\n", autoCloseSeconds)
		time.Sleep(time.Duration(autoCloseSeconds * float64(time.Second)))
		return
	}

	fmt.Print("\nPress Enter to close the browser...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}

func printWindowMetrics(page playwright.Page) {
	metrics, err := page.Evaluate(`() => ({
		outerWidth: window.outerWidth,
		outerHeight: window.outerHeight,
		innerWidth: window.innerWidth,
		innerHeight: window.innerHeight,
		screenWidth: screen.width,
		screenHeight: screen.height,
		availWidth: screen.availWidth,
		availHeight: screen.availHeight
	})`)
	if err != nil {
		log.Printf("window metrics failed: %v", err)
		return
	}
	data, err := json.Marshal(metrics)
	if err != nil {
		log.Printf("window metrics encode failed: %v", err)
		return
	}
	fmt.Printf("Window metrics: %s\n", data)
}
