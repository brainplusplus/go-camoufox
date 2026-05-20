package bidi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNativeBrowserArgs(t *testing.T) {
	args := nativeBrowserArgs([]string{"--foo"}, "profile-dir", true)
	joined := strings.Join(args, " ")
	for _, want := range []string{"--foo", "-headless", "-no-remote", "--remote-debugging-port=0", "-profile profile-dir"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %#v", want, args)
		}
	}
}

func TestNormalizeBiDiEndpoint(t *testing.T) {
	got := normalizeBiDiEndpoint("ws://127.0.0.1:46249")
	if got != "ws://127.0.0.1:46249/session" {
		t.Fatalf("unexpected endpoint: %s", got)
	}
	already := normalizeBiDiEndpoint("ws://127.0.0.1:46249/session")
	if already != "ws://127.0.0.1:46249/session" {
		t.Fatalf("unexpected endpoint with path: %s", already)
	}
}

func TestEndpointFromAddrWildcard(t *testing.T) {
	addr, err := net.ResolveTCPAddr("tcp", "0.0.0.0:9222")
	if err != nil {
		t.Fatal(err)
	}
	got := endpointFromAddr(addr)
	if got != "ws://0.0.0.0:9222" {
		t.Fatalf("unexpected wildcard endpoint: %s", got)
	}
}

func TestWriteUserJS(t *testing.T) {
	dir := t.TempDir()
	err := writeUserJS(dir, map[string]any{
		"media.peerconnection.enabled": false,
		"browser.cache.size":           42,
	}, &ProxyConfig{Server: "socks5://127.0.0.1:1080", Bypass: "localhost"})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "user.js"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{
		`user_pref("remote.active-protocols", 1);`,
		`user_pref("media.peerconnection.enabled", false);`,
		`user_pref("network.proxy.socks", "127.0.0.1");`,
		`user_pref("network.proxy.socks_port", 1080);`,
		`user_pref("network.proxy.socks_version", 5);`,
		`user_pref("network.proxy.no_proxies_on", "localhost");`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("user.js missing %q:\n%s", want, content)
		}
	}
}

func TestServerProxiesBiDiFramesAndCloses(t *testing.T) {
	upstream, upstreamClose := fakeBiDiUpstream(t)
	defer upstreamClose()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, err := Launch(ctx, Options{UpstreamEndpoint: upstream})
	if err != nil {
		t.Fatal(err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(server.Endpoint(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "session.status", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	_, payload, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	if response["id"].(float64) != 1 || response["type"] != "success" {
		t.Fatalf("unexpected response: %s", payload)
	}
	_ = conn.Close()

	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("server did not close after client disconnect")
		default:
			_, _, err := websocket.DefaultDialer.Dial(server.Endpoint(), nil)
			if err != nil {
				return
			}
			time.Sleep(25 * time.Millisecond)
		}
	}
}

func TestLiveCamoufoxBiDiSmoke(t *testing.T) {
	if os.Getenv("GO_CAMOUFOX_LIVE") != "1" {
		t.Skip("set GO_CAMOUFOX_LIVE=1 to run against an installed Camoufox binary")
	}
	executable := os.Getenv("GO_CAMOUFOX_EXECUTABLE")
	if executable == "" {
		t.Fatal("GO_CAMOUFOX_EXECUTABLE is required for live BiDi smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	server, err := Launch(ctx, Options{
		ExecutablePath: executable,
		Headless:       true,
		FirefoxPrefs:   map[string]any{"browser.sessionstore.resume_from_crash": false},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, server.Endpoint(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.WriteJSON(map[string]any{"id": 1, "method": "session.status", "params": map[string]any{}}); err != nil {
		t.Fatal(err)
	}
	var response map[string]any
	if err := conn.ReadJSON(&response); err != nil {
		t.Fatal(err)
	}
	if response["type"] != "success" {
		t.Fatalf("unexpected live response: %#v", response)
	}
	mustWriteBiDi(t, conn, 2, "session.new", map[string]any{"capabilities": map[string]any{}})
	readSuccess(t, conn, 2)
	mustWriteBiDi(t, conn, 3, "browsingContext.create", map[string]any{"type": "tab"})
	create := readSuccess(t, conn, 3)
	result, _ := create["result"].(map[string]any)
	contextID, _ := result["context"].(string)
	if contextID == "" {
		t.Fatalf("missing context id in response: %#v", create)
	}
	mustWriteBiDi(t, conn, 4, "browsingContext.navigate", map[string]any{
		"context": contextID,
		"url":     "data:text/html,<title>go-camoufox</title><h1>ok</h1>",
		"wait":    "complete",
	})
	readSuccess(t, conn, 4)
	mustWriteBiDi(t, conn, 5, "script.evaluate", map[string]any{
		"expression":   "document.querySelector('h1').textContent",
		"target":       map[string]any{"context": contextID},
		"awaitPromise": true,
	})
	evaluated := readSuccess(t, conn, 5)
	result, _ = evaluated["result"].(map[string]any)
	remoteValue, _ := result["result"].(map[string]any)
	if remoteValue["value"] != "ok" {
		t.Fatalf("unexpected script result: %#v", evaluated)
	}
	mustWriteBiDi(t, conn, 6, "browsingContext.close", map[string]any{"context": contextID})
	readSuccess(t, conn, 6)
	mustWriteBiDi(t, conn, 7, "session.end", map[string]any{})
	readSuccess(t, conn, 7)
}

func mustWriteBiDi(t *testing.T, conn *websocket.Conn, id int, method string, params map[string]any) {
	t.Helper()
	if err := conn.WriteJSON(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		t.Fatal(err)
	}
}

func readSuccess(t *testing.T, conn *websocket.Conn, id int) map[string]any {
	t.Helper()
	for {
		var response map[string]any
		if err := conn.ReadJSON(&response); err != nil {
			t.Fatal(err)
		}
		if response["id"] == nil {
			continue
		}
		if int(response["id"].(float64)) != id {
			continue
		}
		if response["type"] != "success" {
			t.Fatalf("unexpected BiDi response for %d: %#v", id, response)
		}
		return response
	}
}

func fakeBiDiUpstream(t *testing.T) (string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			var request map[string]any
			if err := conn.ReadJSON(&request); err != nil {
				return
			}
			_ = conn.WriteJSON(map[string]any{
				"id":     request["id"],
				"type":   "success",
				"result": map[string]any{"ready": true},
			})
		}
	})}
	go func() { _ = server.Serve(listener) }()
	return "ws://" + listener.Addr().String() + "/session", func() {
		_ = server.Close()
		_ = listener.Close()
	}
}
