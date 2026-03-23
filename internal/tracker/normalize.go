package tracker

import "time"

// ParseTime parses an RFC3339 timestamp value from a raw API map field.
// Returns nil for missing, empty, or malformed values.
func ParseTime(v any) *time.Time {
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// ToIntVal coerces a JSON-decoded numeric value to int.
// JSON numbers decode as float64; this handles int, int64, and float64.
func ToIntVal(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
