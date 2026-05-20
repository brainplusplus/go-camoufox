package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	camoufox "github.com/brainplusplus/go-camoufox"
	"github.com/gorilla/websocket"
)

func main() {
	ctx := context.Background()

	headless := camoufox.HeadlessFalse
	built, err := camoufox.BuildLaunchOptions(&camoufox.LaunchOptions{
		Headless: &headless,
	})
	if err != nil {
		log.Fatal(err)
	}
	server, err := camoufox.LaunchServerHandle(ctx, built)
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(server.Endpoint(), nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	send(conn, 1, "session.new", map[string]any{"capabilities": map[string]any{}})
	readResult(conn, 1)

	send(conn, 2, "browsingContext.create", map[string]any{"type": "tab"})
	contextID := readResult(conn, 2)["context"].(string)

	send(conn, 3, "browsingContext.navigate", map[string]any{
		"context": contextID,
		"url":     "https://abrahamjuliot.github.io/creepjs/",
		"wait":    "complete",
	})
	readResult(conn, 3)

	send(conn, 4, "script.evaluate", map[string]any{
		"target":       map[string]any{"context": contextID},
		"awaitPromise": true,
		"expression": `
			await new Promise((resolve) => {
			  let remaining = 300;
			  const timer = setInterval(() => {
			    if (document.querySelector("#creep-results .grade") || --remaining <= 0) {
			      clearInterval(timer);
			      resolve();
			    }
			  }, 100);
			});
			({
			  title: document.title,
			  grade: document.querySelector("#creep-results .grade")?.innerText || null,
			  userAgent: navigator.userAgent
			})
		`,
	})
	evaluated := readResult(conn, 4)
	value := evaluated["result"].(map[string]any)["value"]
	fmt.Printf("%#v\n", value)

	send(conn, 5, "browsingContext.close", map[string]any{"context": contextID})
	readResult(conn, 5)
	send(conn, 6, "session.end", map[string]any{})
	readResult(conn, 6)
}

func send(conn *websocket.Conn, id int, method string, params map[string]any) {
	if err := conn.WriteJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		log.Fatal(err)
	}
}

func readResult(conn *websocket.Conn, id int) map[string]any {
	for {
		var message map[string]any
		if err := conn.ReadJSON(&message); err != nil {
			log.Fatal(err)
		}
		if message["id"] == nil || int(message["id"].(float64)) != id {
			continue
		}
		if message["type"] != "success" {
			data, _ := json.MarshalIndent(message, "", "  ")
			log.Fatalf("BiDi command failed: %s", data)
		}
		result, _ := message["result"].(map[string]any)
		return result
	}
}
