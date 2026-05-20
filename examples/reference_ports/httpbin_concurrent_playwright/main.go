package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	camoufox "github.com/brainplusplus/go-camoufox"
	playwright "github.com/playwright-community/playwright-go"
)

var urls = []string{
	"https://httpbin.org/headers",
	"https://httpbin.org/user-agent",
	"https://httpbin.org/ip",
}

type result struct {
	url  string
	body string
	err  error
}

func main() {
	ctx := context.Background()

	headless := camoufox.HeadlessTrue
	browser, err := camoufox.New(ctx, &camoufox.LaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close(ctx)

	context, err := browser.Browser.NewContext()
	if err != nil {
		log.Fatal(err)
	}
	defer context.Close()

	results := make(chan result, len(urls))
	var wg sync.WaitGroup
	for _, targetURL := range urls {
		wg.Add(1)
		go func(targetURL string) {
			defer wg.Done()
			results <- scrape(context, targetURL)
		}(targetURL)
	}
	wg.Wait()
	close(results)

	for item := range results {
		fmt.Printf("\n--- %s ---\n", item.url)
		if item.err != nil {
			fmt.Println("error:", item.err)
			continue
		}
		fmt.Println(item.body)
	}
}

func scrape(context playwright.BrowserContext, targetURL string) result {
	page, err := context.NewPage()
	if err != nil {
		return result{url: targetURL, err: err}
	}
	defer page.Close()

	if _, err := page.Goto(targetURL); err != nil {
		return result{url: targetURL, err: err}
	}
	body, err := page.InnerText("body")
	if err != nil {
		return result{url: targetURL, err: err}
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return result{url: targetURL, body: body}
}
