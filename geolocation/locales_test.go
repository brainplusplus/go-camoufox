package geolocation

import "testing"

func TestHandleLocalesExplicitAndAll(t *testing.T) {
	config := map[string]any{}
	if err := HandleLocales([]string{"en-US", "en", "fr-FR", "en"}, config); err != nil {
		t.Fatal(err)
	}
	if config["locale:language"] != "en" || config["locale:region"] != "US" {
		t.Fatalf("primary locale not applied: %#v", config)
	}
	if config["locale:all"] != "en-US, en, fr-FR" {
		t.Fatalf("locale:all mismatch: %#v", config)
	}
}

func TestHandleLocaleRegionFromTerritoryInfo(t *testing.T) {
	locale, err := HandleLocale("US", false)
	if err != nil {
		t.Fatal(err)
	}
	if locale.Region != "US" || locale.Language == "" {
		t.Fatalf("unexpected locale for US: %#v", locale)
	}
}

func TestGeolocationAsConfig(t *testing.T) {
	accuracy := 42.0
	config := (Geolocation{
		Locale:    Locale{Language: "en", Region: "US"},
		Longitude: -122.33,
		Latitude:  47.60,
		Timezone:  "America/Los_Angeles",
		Accuracy:  &accuracy,
	}).AsConfig()
	if config["timezone"] != "America/Los_Angeles" || config["locale:region"] != "US" {
		t.Fatalf("geolocation config mismatch: %#v", config)
	}
	if config["geolocation:accuracy"] != accuracy {
		t.Fatalf("accuracy not preserved: %#v", config)
	}
}
