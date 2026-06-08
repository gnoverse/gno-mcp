package budget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyBudget_Small(t *testing.T) {
	r := Apply("hello", "https://gno.land/r/x", false)
	assert.Equal(t, "hello", r.Full, "small content should be returned in full: %+v", r)
	assert.False(t, r.Truncated, "small content should not be truncated: %+v", r)
}

func TestApplyBudget_Large(t *testing.T) {
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", false)
	assert.True(t, r.Truncated, "large content should be truncated: %+v", r)
	assert.Empty(t, r.Full, "large content should not have Full set: %+v", r)
	assert.NotEmpty(t, r.Summary, "expected summary for large content")
}

func TestApplyBudget_SliceRequested(t *testing.T) {
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", true)
	assert.False(t, r.Truncated, "slice requested should not be truncated: %+v", r)
	assert.NotEmpty(t, r.Full, "slice requested should always return full: %+v", r)
}

func TestApplyBudget_NoURL(t *testing.T) {
	big := make([]byte, DefaultBudget+1)
	r := Apply(string(big), "", false)
	require.True(t, r.Truncated, "expected truncation")
	assert.True(t, strings.Contains(r.Summary, "preview omitted") && !strings.Contains(r.Summary, "view at "),
		"empty-URL summary must omit the 'view at' clause: %q", r.Summary)
}
