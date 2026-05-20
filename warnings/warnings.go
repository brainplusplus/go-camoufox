package warnings

import (
	"log"
	"strings"

	"github.com/brainplusplus/go-camoufox/internal/assets"
	"gopkg.in/yaml.v3"
)

var warningMessages map[string]string

func Warn(key string, iKnowWhatImDoing bool) {
	if iKnowWhatImDoing && key != "proxy_without_geoip" {
		return
	}
	msg := message(key)
	if msg == "" {
		msg = key
	}
	log.Printf("camoufox warning [%s]: %s", key, strings.Join(strings.Fields(msg), " "))
}

func WarnManualConfig(config map[string]any) {
	for key := range config {
		switch {
		case strings.HasPrefix(key, "navigator."):
			Warn("navigator", false)
		case strings.HasPrefix(key, "locale:"):
			Warn("locale", false)
		case strings.HasPrefix(key, "geolocation:"):
			Warn("geolocation", false)
		case key == "header.User-Agent":
			Warn("header-ua", false)
		case strings.HasPrefix(key, "screen:") || strings.HasPrefix(key, "window."):
			Warn("viewport", false)
		}
	}
}

func message(key string) string {
	if warningMessages == nil {
		warningMessages = map[string]string{}
		data, err := assets.ReadFile("warnings.yml")
		if err == nil {
			_ = yaml.Unmarshal(data, &warningMessages)
		}
	}
	return warningMessages[key]
}
