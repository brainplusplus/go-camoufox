package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	endpoint := os.Getenv("CAMOUFOX_BIDI_ENDPOINT")
	if endpoint == "" {
		log.Fatal("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")
	}
	if _, err := url.Parse(endpoint); err != nil {
		log.Fatal(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	send(conn, 1, "session.status", map[string]any{})
	fmt.Printf("status: %#v\n", readResult(conn, 1))

	send(conn, 2, "session.new", map[string]any{"capabilities": map[string]any{}})
	readResult(conn, 2)

	send(conn, 3, "browsingContext.create", map[string]any{"type": "tab"})
	create := readResult(conn, 3)
	contextID := create["context"].(string)

	send(conn, 4, "browsingContext.navigate", map[string]any{
		"context": contextID,
		"url":     "data:text/html,<title>go-camoufox</title><h1>hello from Go</h1>",
		"wait":    "complete",
	})
	readResult(conn, 4)

	send(conn, 5, "script.evaluate", map[string]any{
		"expression":   "document.querySelector('h1').textContent",
		"target":       map[string]any{"context": contextID},
		"awaitPromise": true,
	})
	evaluated := readResult(conn, 5)
	value := evaluated["result"].(map[string]any)["value"]
	fmt.Println(value)

	send(conn, 6, "browsingContext.close", map[string]any{"context": contextID})
	readResult(conn, 6)
	send(conn, 7, "session.end", map[string]any{})
	readResult(conn, 7)
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
