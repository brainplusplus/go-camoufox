package warnings

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestWarnManualConfigReferenceKeys(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)

	WarnManualConfig(map[string]any{
		"navigator.language":      "en-US",
		"headers.User-Agent":      "Mozilla/5.0",
		"timezone":                "America/New_York",
		"screen.width":            1920,
		"document.body.clientTop": 0,
	})
	output := buf.String()
	for _, key := range []string{"locale", "header-ua", "geolocation", "viewport"} {
		if !strings.Contains(output, "camoufox warning ["+key+"]") {
			t.Fatalf("missing warning %s in:\n%s", key, output)
		}
	}
}

func TestWarnSuppressionParity(t *testing.T) {
	var buf bytes.Buffer
	original := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(original)

	Warn("block_images", true)
	if buf.Len() != 0 {
		t.Fatalf("expected suppressible warning to be quiet, got %q", buf.String())
	}
	Warn("proxy_without_geoip", true)
	if !strings.Contains(buf.String(), "proxy_without_geoip") {
		t.Fatalf("expected proxy warning to remain unsuppressed, got %q", buf.String())
	}
}
