package config

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/brainplusplus/go-camoufox/internal/assets"
)

type propertySchema struct {
	Property string `json:"property"`
	Type     string `json:"type"`
}

func Validate(config map[string]any) error {
	types, err := LoadPropertyTypes()
	if err != nil {
		return err
	}
	for key, value := range config {
		expected := types[key]
		if expected == "" {
			continue
		}
		if !ValidateType(value, expected) {
			return fmt.Errorf("invalid type for property %s: expected %s, got %T", key, expected, value)
		}
	}
	return nil
}

func LoadPropertyTypes() (map[string]string, error) {
	data, err := assets.ReadFile("properties.json")
	if err != nil {
		return nil, err
	}
	var schemas []propertySchema
	if err := json.Unmarshal(data, &schemas); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(schemas))
	for _, schema := range schemas {
		out[schema.Property] = schema.Type
	}
	return out, nil
}

func ValidateType(value any, expected string) bool {
	switch expected {
	case "str":
		_, ok := value.(string)
		return ok
	case "int":
		return isInteger(value)
	case "uint":
		number, ok := numberValue(value)
		return ok && math.Trunc(number) == number && number >= 0
	case "double":
		_, ok := numberValue(value)
		return ok
	case "bool":
		_, ok := value.(bool)
		return ok
	case "array":
		switch value.(type) {
		case []any, []string, []int, []float64:
			return true
		default:
			return false
		}
	case "dict":
		switch value.(type) {
		case map[string]any, map[string]string:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isInteger(value any) bool {
	number, ok := numberValue(value)
	return ok && math.Trunc(number) == number
}

func numberValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	default:
		return 0, false
	}
}
