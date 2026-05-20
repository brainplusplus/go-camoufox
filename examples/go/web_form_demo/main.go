package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

func main() {
	endpoint := os.Getenv("CAMOUFOX_BIDI_ENDPOINT")
	if endpoint == "" {
		log.Fatal("set CAMOUFOX_BIDI_ENDPOINT, for example ws://127.0.0.1:50123/session")
	}

	appURL, shutdown := startDemoSite()
	defer shutdown()

	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
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
		"url":     appURL,
		"wait":    "complete",
	})
	readResult(conn, 3)

	send(conn, 4, "script.evaluate", map[string]any{
		"target":       map[string]any{"context": contextID},
		"awaitPromise": true,
		"expression": `
			document.querySelector("#item").value = "Phase 4 BiDi demo";
			document.querySelector("#add").click();
			document.querySelector("#item").value = "Go form automation";
			document.querySelector("#add").click();
			Array.from(document.querySelectorAll("li")).map((li) => li.textContent).join(" | ");
		`,
	})
	evaluated := readResult(conn, 4)
	value := evaluated["result"].(map[string]any)["value"]
	fmt.Println(value)

	send(conn, 5, "browsingContext.close", map[string]any{"context": contextID})
	readResult(conn, 5)
	send(conn, 6, "session.end", map[string]any{})
	readResult(conn, 6)
}

func startDemoSite() (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html>
<title>go-camoufox form demo</title>
<h1>Task list</h1>
<input id="item" aria-label="Item">
<button id="add">Add</button>
<ul id="items"></ul>
<script>
document.querySelector("#add").addEventListener("click", () => {
  const input = document.querySelector("#item");
  const li = document.createElement("li");
  li.textContent = input.value;
  document.querySelector("#items").append(li);
  input.value = "";
});
</script>`)
	})
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	return "http://" + listener.Addr().String(), func() {
		_ = server.Close()
		_ = listener.Close()
	}
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
