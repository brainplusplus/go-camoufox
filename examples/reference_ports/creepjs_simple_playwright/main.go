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

const acceptEncoding = "identity"

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

	page, err := browser.Browser.NewPage(playwright.BrowserNewPageOptions{
		ExtraHttpHeaders: map[string]string{"accept-encoding": acceptEncoding},
	})
	if err != nil {
		log.Fatal(err)
	}
	if _, err := page.Goto("https://abrahamjuliot.github.io/creepjs/"); err != nil {
		log.Fatal(err)
	}

	fmt.Print("Press Enter to close...")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
}
