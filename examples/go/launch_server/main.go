package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"

	camoufox "github.com/brainplusplus/go-camoufox"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	headless := camoufox.HeadlessTrue
	opts := &camoufox.LaunchOptions{
		Headless:         &headless,
		OS:               []string{"windows"},
		ExcludeAddons:    []camoufox.DefaultAddon{camoufox.AddonUBO},
		IKnowWhatImDoing: boolPtr(true),
	}

	built, err := camoufox.BuildLaunchOptions(opts)
	if err != nil {
		log.Fatal(err)
	}
	server, err := camoufox.LaunchServerHandle(ctx, built)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	fmt.Println(server.Endpoint())
	<-ctx.Done()
}

func boolPtr(value bool) *bool {
	return &value
}
