package geolocation

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Proxy struct {
	Server   string
	Username string
	Password string
	Bypass   string
}

var proxyServerRe = regexp.MustCompile(`^(?:(?P<schema>\w+)://)?(?P<url>.*?)(?:\:(?P<port>\d+))?$`)

func (p Proxy) AsString() (string, error) {
	match := proxyServerRe.FindStringSubmatch(p.Server)
	if match == nil {
		return "", fmt.Errorf("invalid proxy server: %s", p.Server)
	}
	schema := match[1]
	host := match[2]
	port := match[3]
	if schema == "" {
		schema = "http"
	}
	if host == "" {
		return "", fmt.Errorf("invalid proxy server: %s", p.Server)
	}
	var b strings.Builder
	b.WriteString(schema)
	b.WriteString("://")
	if p.Username != "" {
		b.WriteString(url.QueryEscape(p.Username))
		if p.Password != "" {
			b.WriteByte(':')
			b.WriteString(url.QueryEscape(p.Password))
		}
		b.WriteByte('@')
	}
	b.WriteString(host)
	if port != "" {
		b.WriteByte(':')
		b.WriteString(port)
	}
	return b.String(), nil
}

func ValidIPv4(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() != nil
}

func ValidIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() == nil && parsed.To16() != nil
}

func ValidateIP(ip string) error {
	if !ValidIPv4(ip) && !ValidIPv6(ip) {
		return fmt.Errorf("invalid IP address: %s", ip)
	}
	return nil
}

func IsPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	return parsed.IsPrivate() || parsed.IsLoopback() || parsed.IsLinkLocalUnicast() || parsed.IsLinkLocalMulticast() || parsed.IsUnspecified()
}

func ValidatePublicIP(ip string) error {
	if err := ValidateIP(ip); err != nil {
		return err
	}
	if IsPrivateIP(ip) {
		return fmt.Errorf("IP address is not public: %s", ip)
	}
	return nil
}

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// ipCall represents an in-flight or completed PublicIP lookup.
type ipCall struct {
	done chan struct{}
	val  string
	err  error
}

var (
	PublicIPURLs = []string{
		"https://api.ipify.org",
		"https://checkip.amazonaws.com",
		"https://ipinfo.io/ip",
		"https://icanhazip.com",
		"https://ifconfig.co/ip",
		"https://ipecho.net/plain",
	}
	defaultPublicIPClient          = &http.Client{Timeout: 5 * time.Second}
	PublicIPClient        HTTPDoer = defaultPublicIPClient

	publicIPMu     sync.Mutex
	publicIPCache  = map[string]string{}
	publicIPFlight = map[string]*ipCall{}
)

func PublicIP(ctx context.Context, proxyString string) (string, error) {
	publicIPMu.Lock()
	// Fast path: already cached.
	if value, ok := publicIPCache[proxyString]; ok {
		publicIPMu.Unlock()
		return value, nil
	}
	// Deduplicate: if another goroutine is already fetching for this proxy, wait for it.
	if call, ok := publicIPFlight[proxyString]; ok {
		publicIPMu.Unlock()
		select {
		case <-call.done:
			return call.val, call.err
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	// This goroutine owns the fetch.
	call := &ipCall{done: make(chan struct{})}
	publicIPFlight[proxyString] = call
	publicIPMu.Unlock()

	// Ensure waiters are always unblocked, even on panic.
	defer func() {
		publicIPMu.Lock()
		if call.err == nil && call.val != "" {
			publicIPCache[proxyString] = call.val
		}
		delete(publicIPFlight, proxyString)
		publicIPMu.Unlock()
		close(call.done)
	}()

	call.val, call.err = fetchPublicIP(ctx, proxyString)
	return call.val, call.err
}

func fetchPublicIP(ctx context.Context, proxyString string) (string, error) {
	var lastErr error
	for _, endpoint := range PublicIPURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := publicIPClient(proxyString).Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 256))
		closeErr := resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if closeErr != nil {
			lastErr = closeErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("public IP endpoint %s returned %s", endpoint, resp.Status)
			continue
		}
		ip := strings.TrimSpace(string(body))
		if err := ValidatePublicIP(ip); err != nil {
			lastErr = err
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("failed to get IP address: %w", lastErr)
}

func publicIPClient(proxyString string) HTTPDoer {
	if proxyString == "" || PublicIPClient != defaultPublicIPClient {
		return PublicIPClient
	}
	proxyURL, err := url.Parse(proxyString)
	if err != nil {
		return PublicIPClient
	}
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
}
