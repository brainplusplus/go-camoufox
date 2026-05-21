package camoufox

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

var managedLaunchExtraKeys = map[string]struct{}{
	"args":               {},
	"env":                {},
	"executablePath":     {},
	"executable_path":    {},
	"firefoxUserPrefs":   {},
	"firefox_user_prefs": {},
	"headless":           {},
	"proxy":              {},
}

var managedPersistentExtraKeys = map[string]struct{}{
	"screen":      {},
	"viewport":    {},
	"userAgent":   {},
	"user_agent":  {},
	"timezoneId":  {},
	"timezone_id": {},
	"locale":      {},
}

func applyLaunchExtra(target any, extra map[string]any) error {
	return applyExtraOptions(target, extra, managedLaunchExtraKeys)
}

func applyPersistentExtra(target any, extra map[string]any) error {
	managed := map[string]struct{}{}
	for key := range managedLaunchExtraKeys {
		managed[key] = struct{}{}
	}
	for key := range managedPersistentExtraKeys {
		managed[key] = struct{}{}
	}
	return applyExtraOptions(target, extra, managed)
}

func applyExtraOptions(target any, extra map[string]any, managed map[string]struct{}) error {
	if len(extra) == 0 {
		return nil
	}
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Pointer || targetValue.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("extra options target must be a pointer to struct")
	}
	allowed := jsonFieldNames(targetValue.Elem().Type())
	normalized := map[string]any{}
	for key, value := range extra {
		jsonKey, ok := allowed[normalizeOptionKey(key)]
		if !ok {
			return fmt.Errorf("unsupported Playwright option %q", key)
		}
		if _, ok := managed[jsonKey]; ok {
			return fmt.Errorf("Playwright option %q is managed by go-camoufox; use the typed LaunchOptions field instead", key)
		}
		if _, ok := managed[normalizeOptionKey(key)]; ok {
			return fmt.Errorf("Playwright option %q is managed by go-camoufox; use the typed LaunchOptions field instead", key)
		}
		normalized[jsonKey] = value
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	overlay := reflect.New(targetValue.Elem().Type())
	if err := json.Unmarshal(payload, overlay.Interface()); err != nil {
		return err
	}
	mergeNonZeroStruct(targetValue.Elem(), overlay.Elem())
	return nil
}

func jsonFieldNames(t reflect.Type) map[string]string {
	out := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		name := field.Name
		if tag := field.Tag.Get("json"); tag != "" {
			name, _, _ = strings.Cut(tag, ",")
		}
		if name == "" || name == "-" {
			continue
		}
		out[normalizeOptionKey(name)] = name
		out[normalizeOptionKey(field.Name)] = name
	}
	return out
}

func normalizeOptionKey(key string) string {
	key = strings.TrimSpace(key)
	key = strings.ReplaceAll(key, "_", "")
	key = strings.ReplaceAll(key, "-", "")
	return strings.ToLower(key)
}

func mergeNonZeroStruct(dst, src reflect.Value) {
	for i := 0; i < src.NumField(); i++ {
		value := src.Field(i)
		if value.IsZero() {
			continue
		}
		field := dst.Field(i)
		if field.CanSet() {
			field.Set(value)
		}
	}
}
