package engine

import "fmt"

// VarSpec describes a typed slot: a world variable, entity attribute,
// relationship attribute, or action parameter.
type VarSpec struct {
	Type    string   `json:"type"` // int|float|bool|string|enum|ref
	Default any      `json:"default,omitempty"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
	Values  []string `json:"values,omitempty"`  // enum members
	RefType string   `json:"refType,omitempty"` // ref target entity type
}

// toFloat coerces JSON-decoded numbers to float64.
func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	default:
		return 0, false
	}
}

// ValidateValue checks v against spec's type, bounds, and enum membership.
func ValidateValue(spec VarSpec, v any) error {
	switch spec.Type {
	case "int", "float":
		f, ok := toFloat(v)
		if !ok {
			return fmt.Errorf("expected number, got %T", v)
		}
		if spec.Type == "int" && f != float64(int64(f)) {
			return fmt.Errorf("expected whole number, got %v", f)
		}
		if spec.Min != nil && f < *spec.Min {
			return fmt.Errorf("value %v below min %v", f, *spec.Min)
		}
		if spec.Max != nil && f > *spec.Max {
			return fmt.Errorf("value %v above max %v", f, *spec.Max)
		}
	case "bool":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("expected bool, got %T", v)
		}
	case "string", "ref":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("expected string, got %T", v)
		}
	case "enum":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("expected enum string, got %T", v)
		}
		for _, m := range spec.Values {
			if s == m {
				return nil
			}
		}
		return fmt.Errorf("value %q not in enum %v", s, spec.Values)
	default:
		return fmt.Errorf("unknown type %q", spec.Type)
	}
	return nil
}

// DefaultValue returns spec.Default if set, else the zero value for the type.
func DefaultValue(spec VarSpec) any {
	if spec.Default != nil {
		return spec.Default
	}
	switch spec.Type {
	case "int", "float":
		return float64(0)
	case "bool":
		return false
	case "string", "ref":
		return ""
	case "enum":
		if len(spec.Values) > 0 {
			return spec.Values[0]
		}
		return ""
	}
	return nil
}
