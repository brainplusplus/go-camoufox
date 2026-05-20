package geolocation

import "testing"

func TestLoadGeoIPReposDefault(t *testing.T) {
	repos, defaultName, err := LoadGeoIPRepos()
	if err != nil {
		t.Fatal(err)
	}
	if defaultName != "MaxMind GeoLite2" {
		t.Fatalf("unexpected default GeoIP source: %q", defaultName)
	}
	if len(repos) < 2 {
		t.Fatalf("expected multiple GeoIP repos, got %d", len(repos))
	}
	if _, err := GetGeoIPConfigByName("GeoIP AIO by daijro"); err != nil {
		t.Fatal(err)
	}
}

func TestFindInDottedPath(t *testing.T) {
	data := map[string]any{
		"country": map[string]any{"iso_code": "US"},
		"location": map[string]any{
			"longitude": -122.33,
			"latitude":  47.60,
		},
	}
	if got := findIn(data, "country.iso_code"); got != "US" {
		t.Fatalf("unexpected nested value: %#v", got)
	}
	if got := findIn(data, "location.longitude"); got != -122.33 {
		t.Fatalf("unexpected longitude: %#v", got)
	}
}
