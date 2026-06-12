package counter

import "strconv"

var total int

// Increment adds 1 to the running total and returns the new value.
// Declared `cur realm` so MsgCall (and session-signed MsgCall) can invoke it.
func Increment(cur realm) int {
	total++
	return total
}

// Total returns the current running total without modifying it.
func Total() int {
	return total
}

// Render returns a markdown summary of the current total.
// gno_render exercises it end-to-end in the e2e read-tool scenario.
func Render(path string) string {
	return "# Counter\n\nTotal: " + strconv.Itoa(total)
}
