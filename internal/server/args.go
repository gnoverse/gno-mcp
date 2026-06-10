package server

import "fmt"

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
