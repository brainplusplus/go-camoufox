package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	camoufox "github.com/brainplusplus/go-camoufox"
	playwright "github.com/playwright-community/playwright-go"
)

func main() {
	ctx := context.Background()

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

	if _, err := page.Goto("https://abrahamjuliot.github.io/creepjs/"); err != nil {
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

	scoreEl, err := page.QuerySelector("#creep-results .grade")
	if err != nil {
		log.Fatal(err)
	}
	if scoreEl != nil {
		score, err := scoreEl.InnerText()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("CreepJS trust grade: %s\n", score)
	} else {
		fmt.Println("Score element not found - page may still be loading.")
	}

	ua, err := page.Evaluate("navigator.userAgent")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("User-Agent: %v\n", ua)

	fmt.Print("\nPress Enter to close the browser...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
