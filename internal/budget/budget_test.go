package budget

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyBudget_Small(t *testing.T) {
	r := Apply("hello", "https://gno.land/r/x", DefaultBudget)
	assert.Equal(t, "hello", r.Full, "small content should be returned in full: %+v", r)
	assert.False(t, r.Truncated, "small content should not be truncated: %+v", r)
}

func TestApplyBudget_LargeOverDefault(t *testing.T) {
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", DefaultBudget)
	assert.True(t, r.Truncated, "large content should be truncated: %+v", r)
	assert.Empty(t, r.Full, "large content should not have Full set: %+v", r)
	assert.NotEmpty(t, r.Summary, "expected summary for large content")
}

func TestApplyBudget_ExplicitTierAdmitsRealFiles(t *testing.T) {
	// 5000 bytes blows the default tier but is an ordinary single-file size;
	// an explicit request (full=true / symbols) gets the high tier.
	large := string(make([]byte, 5000))
	r := Apply(large, "https://gno.land/r/x", ExplicitBudget)
	assert.False(t, r.Truncated, "explicit tier must admit ordinary file sizes: %+v", r)
	assert.NotEmpty(t, r.Full)
}

func TestApplyBudget_ExplicitTierStillCaps(t *testing.T) {
	huge := string(make([]byte, ExplicitBudget+1))
	r := Apply(huge, "https://gno.land/r/x", ExplicitBudget)
	assert.True(t, r.Truncated, "the explicit tier is a higher ceiling, not a bypass: %+v", r)
}

func TestApplyBudget_NoURL(t *testing.T) {
	big := make([]byte, DefaultBudget+1)
	r := Apply(string(big), "", DefaultBudget)
	require.True(t, r.Truncated, "expected truncation")
	assert.True(t, strings.Contains(r.Summary, "preview omitted") && !strings.Contains(r.Summary, "view at "),
		"empty-URL summary must omit the 'view at' clause: %q", r.Summary)
}

func TestApplyBudget_TierOrdering(t *testing.T) {
	assert.Greater(t, ExplicitBudget, DefaultBudget,
		"explicit requests must get more room than broad sweeps")
}
