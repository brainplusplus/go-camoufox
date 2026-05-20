package bidi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultReadyTimeout = 30 * time.Second

var endpointPattern = regexp.MustCompile(`ws://[^\s]+`)

type Options struct {
	ExecutablePath string
	Args           []string
	Env            map[string]string
	FirefoxPrefs   map[string]any
	Headless       bool
	Proxy          *ProxyConfig
	Stdout         io.Writer
	Stderr         io.Writer
	ReadyTimeout   time.Duration
	ListenAddr     string

	UpstreamEndpoint string
}

type ProxyConfig struct {
	Server   string
	Bypass   string
	Username string
	Password string
}

type Server struct {
	endpoint string

	listener net.Listener
	http     *http.Server
	cmd      *exec.Cmd
	cancel   context.CancelFunc

	tempDir string
	stderr  io.Writer
	done    chan struct{}

	mu     sync.Mutex
	closed bool
}

func Launch(ctx context.Context, opts Options) (*Server, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	runCtx, cancel := context.WithCancel(ctx)
	server := &Server{cancel: cancel, stderr: opts.Stderr, done: make(chan struct{})}

	upstream := opts.UpstreamEndpoint
	var err error
	if upstream == "" {
		upstream, err = server.launchBrowser(runCtx, opts)
		if err != nil {
			cancel()
			_ = server.Close()
			return nil, err
		}
	}
	if err := server.listen(runCtx, upstream, opts.ListenAddr); err != nil {
		cancel()
		_ = server.Close()
		return nil, err
	}
	go func() {
		<-runCtx.Done()
		_ = server.Close()
	}()
	return server, nil
}

func (s *Server) Endpoint() string {
	if s == nil {
		return ""
	}
	return s.endpoint
}

func (s *Server) Done() <-chan struct{} {
	if s == nil {
		closed := make(chan struct{})
		close(closed)
		return closed
	}
	return s.done
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.done)
	cancel := s.cancel
	httpServer := s.http
	listener := s.listener
	cmd := s.cmd
	tempDir := s.tempDir
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	var closeErr error
	if httpServer != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 3*time.Second)
		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			closeErr = err
		}
		cancelShutdown()
	}
	if listener != nil {
		if err := listener.Close(); closeErr == nil && err != nil && !errors.Is(err, net.ErrClosed) {
			closeErr = err
		}
	}
	if cmd != nil && cmd.Process != nil {
		if err := terminateProcess(cmd); closeErr == nil && err != nil {
			closeErr = err
		}
	}
	if tempDir != "" {
		if err := os.RemoveAll(tempDir); closeErr == nil && err != nil {
			closeErr = err
		}
	}
	return closeErr
}

func (s *Server) launchBrowser(ctx context.Context, opts Options) (string, error) {
	if opts.ExecutablePath == "" {
		return "", errors.New("executable path is required")
	}
	profile, err := os.MkdirTemp("", "go-camoufox-bidi-profile-*")
	if err != nil {
		return "", err
	}
	s.tempDir = profile
	if err := writeUserJS(profile, opts.FirefoxPrefs, opts.Proxy); err != nil {
		return "", err
	}

	args := nativeBrowserArgs(opts.Args, profile, opts.Headless)
	cmd := exec.CommandContext(ctx, opts.ExecutablePath, args...)
	cmd.Env = envSlice(opts.Env)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", err
	}

	endpointCh := make(chan string, 1)
	errCh := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return "", err
	}
	s.cmd = cmd
	go scanBrowserOutput(stdout, opts.Stdout, endpointCh)
	go scanBrowserOutput(stderr, opts.Stderr, endpointCh)
	go func() {
		errCh <- cmd.Wait()
	}()

	timeout := opts.ReadyTimeout
	if timeout == 0 {
		timeout = defaultReadyTimeout
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case endpoint := <-endpointCh:
		return normalizeBiDiEndpoint(endpoint), nil
	case err := <-errCh:
		if err == nil {
			err = errors.New("browser exited before reporting a WebDriver BiDi endpoint")
		}
		return "", err
	case <-timer.C:
		return "", fmt.Errorf("timed out after %s waiting for WebDriver BiDi endpoint", timeout)
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func normalizeBiDiEndpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Path != "" {
		return endpoint
	}
	parsed.Path = "/session"
	return parsed.String()
}

func (s *Server) listen(ctx context.Context, upstream, listenAddr string) error {
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.endpoint = endpointFromAddr(listener.Addr()) + "/session"

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.handleWebSocket(ctx, upstream, w, r)
	})
	httpServer := &http.Server{Handler: mux}
	s.http = httpServer
	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && s.stderr != nil {
			_, _ = fmt.Fprintf(s.stderr, "BiDi server stopped: %v\n", err)
		}
	}()
	return nil
}

func endpointFromAddr(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "ws://" + addr.String()
	}
	if host == "" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	return "ws://" + net.JoinHostPort(host, port)
}

func (s *Server) handleWebSocket(ctx context.Context, upstream string, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	client, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer client.Close()

	remote, _, err := websocket.DefaultDialer.DialContext(ctx, upstream, nil)
	if err != nil {
		_ = client.WriteJSON(map[string]any{
			"type":    "error",
			"error":   "session not created",
			"message": err.Error(),
		})
		_ = s.Close()
		return
	}
	defer remote.Close()

	done := make(chan struct{}, 2)
	go proxyMessages(client, remote, done)
	go proxyMessages(remote, client, done)
	select {
	case <-done:
	case <-ctx.Done():
	}
	_ = s.Close()
}

func proxyMessages(src, dst *websocket.Conn, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()
	for {
		messageType, payload, err := src.ReadMessage()
		if err != nil {
			return
		}
		if err := dst.WriteMessage(messageType, payload); err != nil {
			return
		}
	}
}

func scanBrowserOutput(reader io.Reader, mirror io.Writer, endpointCh chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if mirror != nil {
			_, _ = fmt.Fprintln(mirror, line)
		}
		if endpoint := endpointPattern.FindString(line); endpoint != "" {
			select {
			case endpointCh <- endpoint:
			default:
			}
		}
	}
}

func nativeBrowserArgs(userArgs []string, profile string, headless bool) []string {
	args := append([]string(nil), userArgs...)
	if headless && !hasArg(args, "-headless") && !hasArg(args, "--headless") {
		args = append(args, "-headless")
	}
	if !hasArg(args, "-no-remote") && !hasArg(args, "--no-remote") {
		args = append(args, "-no-remote")
	}
	if !hasArgPrefix(args, "--remote-debugging-port") {
		args = append(args, "--remote-debugging-port=0")
	}
	if !hasArg(args, "-profile") && !hasArg(args, "--profile") {
		args = append(args, "-profile", profile)
	}
	return args
}

func hasArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}

func hasArgPrefix(args []string, prefix string) bool {
	for _, arg := range args {
		if arg == prefix || strings.HasPrefix(arg, prefix+"=") {
			return true
		}
	}
	return false
}

func writeUserJS(profile string, prefs map[string]any, proxy *ProxyConfig) error {
	merged := map[string]any{
		"remote.active-protocols":           1,
		"devtools.chrome.enabled":           true,
		"devtools.debugger.remote-enabled":  true,
		"browser.shell.checkDefaultBrowser": false,
	}
	for key, value := range proxyPrefs(proxy) {
		merged[key] = value
	}
	for key, value := range prefs {
		merged[key] = value
	}

	keys := make([]string, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	for _, key := range keys {
		value, err := prefLiteral(merged[key])
		if err != nil {
			return fmt.Errorf("firefox pref %s: %w", key, err)
		}
		builder.WriteString("user_pref(")
		builder.WriteString(strconv.Quote(key))
		builder.WriteString(", ")
		builder.WriteString(value)
		builder.WriteString(");\n")
	}
	return os.WriteFile(filepath.Join(profile, "user.js"), []byte(builder.String()), 0o644)
}

func prefLiteral(value any) (string, error) {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed), nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int:
		return strconv.Itoa(typed), nil
	case int8, int16, int32, int64:
		return fmt.Sprintf("%d", typed), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", typed), nil
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unsupported value type %T", value)
	}
}

func proxyPrefs(proxy *ProxyConfig) map[string]any {
	if proxy == nil || strings.TrimSpace(proxy.Server) == "" {
		return nil
	}
	raw := proxy.Server
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return nil
	}
	port := 0
	if parsed.Port() != "" {
		port, _ = strconv.Atoi(parsed.Port())
	}
	out := map[string]any{
		"network.proxy.type":                 1,
		"network.proxy.share_proxy_settings": false,
	}
	if proxy.Bypass != "" {
		out["network.proxy.no_proxies_on"] = proxy.Bypass
	}
	switch strings.ToLower(parsed.Scheme) {
	case "socks", "socks4":
		out["network.proxy.socks"] = parsed.Hostname()
		out["network.proxy.socks_port"] = port
		out["network.proxy.socks_version"] = 4
	case "socks5":
		out["network.proxy.socks"] = parsed.Hostname()
		out["network.proxy.socks_port"] = port
		out["network.proxy.socks_version"] = 5
	case "https":
		out["network.proxy.ssl"] = parsed.Hostname()
		out["network.proxy.ssl_port"] = port
	default:
		out["network.proxy.http"] = parsed.Hostname()
		out["network.proxy.http_port"] = port
	}
	return out
}

func envSlice(env map[string]string) []string {
	if env == nil {
		return os.Environ()
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+env[key])
	}
	return out
}
