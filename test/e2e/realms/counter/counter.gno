package counter

import "fmt"

var total int

// Increment adds 1 to the running total and returns the new value.
func Increment() int {
	total++
	return total
}

// Total returns the current running total without modifying it.
func Total() int {
	return total
}

// Render returns a markdown summary of the current total.
// Used by Section A1 of PROTOCOL.md to exercise gno_render end-to-end.
func Render(path string) string {
	return fmt.Sprintf("# Counter\n\nTotal: %d", total)
}
