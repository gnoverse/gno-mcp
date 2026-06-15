package server

import (
	"encoding/json"
	"fmt"
)

// StringArg pulls a typed string from the schema-validated args map.
// Missing key returns ("", nil); required-vs-optional is the caller's
// concern. Present but wrong type returns an error.
func StringArg(args map[string]any, name string) (string, error) {
	raw, present := args[name]
	if !present {
		return "", nil
	}
	v, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s: expected string, got %T", name, raw)
	}
	return v, nil
}

// BoolArg pulls a typed bool from the args map.
// Missing key returns (false, nil). Present but wrong type returns an error.
func BoolArg(args map[string]any, name string) (bool, error) {
	raw, present := args[name]
	if !present {
		return false, nil
	}
	v, ok := raw.(bool)
	if !ok {
		return false, fmt.Errorf("%s: expected bool, got %T", name, raw)
	}
	return v, nil
}

// Int64Arg pulls a whole-number int64 from the args map. Missing key returns
// (0, nil). JSON numbers arrive as float64 (or json.Number); a non-integral or
// out-of-range value returns an error.
func Int64Arg(args map[string]any, name string) (int64, error) {
	raw, present := args[name]
	if !present {
		return 0, nil
	}
	switch v := raw.(type) {
	case float64:
		// JSON numbers decode to float64, which loses integer precision above
		// 2^53; such a value arrives already-rounded and cannot be detected here.
		// Callers needing exact large integers must decode with UseNumber().
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("%s: expected a whole number, got %v", name, v)
		}
		return int64(v), nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("%s: expected a whole number, got %q", name, v.String())
		}
		return n, nil
	default:
		return 0, fmt.Errorf("%s: expected a number, got %T", name, raw)
	}
}

// StringSliceArg pulls a []string from the args map.
// Missing key returns (nil, nil). Present value must be []any with every
// element a string; a non-string element returns an error.
func StringSliceArg(args map[string]any, name string) ([]string, error) {
	raw, present := args[name]
	if !present {
		return nil, nil
	}
	rawSlice, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: expected array, got %T", name, raw)
	}
	out := make([]string, len(rawSlice))
	for i, elem := range rawSlice {
		s, ok := elem.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d]: expected string, got %T", name, i, elem)
		}
		out[i] = s
	}
	return out, nil
}
