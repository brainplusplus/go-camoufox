package config

import "testing"

func TestValidateConfigUsesEmbeddedProperties(t *testing.T) {
	if err := Validate(map[string]any{
		"navigator.userAgent":           "Mozilla/5.0",
		"navigator.hardwareConcurrency": 8,
		"navigator.cookieEnabled":       true,
		"navigator.languages":           []string{"en-US", "en"},
		"unknown.patch":                 struct{}{},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestValidateConfigRejectsWrongTypes(t *testing.T) {
	err := Validate(map[string]any{
		"navigator.hardwareConcurrency": -1,
	})
	if err == nil {
		t.Fatal("expected uint validation error")
	}
}
