package geolocation

import "testing"

func TestProxyAsString(t *testing.T) {
	proxy := Proxy{Server: "socks5://127.0.0.1:1080", Username: "user", Password: "pass"}
	got, err := proxy.AsString()
	if err != nil {
		t.Fatal(err)
	}
	want := "socks5://user:pass@127.0.0.1:1080"
	if got != want {
		t.Fatalf("proxy string mismatch: got %q want %q", got, want)
	}
}

func TestProxyAsStringDefaultsSchema(t *testing.T) {
	got, err := (Proxy{Server: "example.com:8080"}).AsString()
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.com:8080" {
		t.Fatalf("unexpected proxy string: %q", got)
	}
}

func TestIPValidation(t *testing.T) {
	for _, ip := range []string{"8.8.8.8", "2001:4860:4860::8888"} {
		if err := ValidatePublicIP(ip); err != nil {
			t.Fatalf("expected %s to be public: %v", ip, err)
		}
	}
	for _, ip := range []string{"999.1.1.1", "127.0.0.1", "10.0.0.1", "fc00::1"} {
		if err := ValidatePublicIP(ip); err == nil {
			t.Fatalf("expected %s to be rejected", ip)
		}
	}
}
