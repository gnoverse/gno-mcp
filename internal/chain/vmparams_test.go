package chain

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures are vm/qpkgmeta_json answers captured from onyx-1.
const (
	pkgMetaInert = `{"path":"gno.land/r/g1ezf973jrnr23xkul0pleauz00ufuf0l6vjc0x7/parked","status":"inert","creator":"g1ezf973jrnr23xkul0pleauz00ufuf0l6vjc0x7","height":26404,"max_deposit":"5000000ugnot","reason":"waiting for a package approver to enable it","pending":true}`
	pkgMetaLive  = `{"path":"gno.land/r/sys/names","status":"live","creator":"g1r929wt2qplfawe4lvqv9zuwfdcz4vxdun7qh8l"}`
	pkgMetaGone  = `{"path":"gno.land/r/nope/absent","status":"absent"}`
	// A redeploy parked over a live private realm: callable, with a submission pending.
	pkgMetaLivePending = `{"path":"gno.land/r/x/priv","status":"live","creator":"g1x","reason":"a live private package at this path was deployed by a different address, so this submission can never be enabled","pending":true}`
)

func TestDecodePackageMeta(t *testing.T) {
	cases := map[string]struct {
		raw  string
		want PackageMeta
		live bool
	}{
		"inert":        {pkgMetaInert, PackageMeta{Status: PackageInert, Reason: "waiting for a package approver to enable it", Pending: true}, false},
		"live":         {pkgMetaLive, PackageMeta{Status: PackageLive}, true},
		"absent":       {pkgMetaGone, PackageMeta{Status: PackageAbsent}, false},
		"live-pending": {pkgMetaLivePending, PackageMeta{Status: PackageLive, Reason: "a live private package at this path was deployed by a different address, so this submission can never be enabled", Pending: true}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := decodePackageMeta([]byte(tc.raw))
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.live, got.Live(), "a pending submission means the new code is not the live one")
		})
	}
}

func TestDecodePackageMeta_boundsTheReason(t *testing.T) {
	// A hostile node reporting an allowlisted chain-id controls the reason. The
	// cut lands inside a multi-byte rune to prove the bound keeps it valid UTF-8.
	reason := strings.Repeat("a", maxPackageReason-1) + "é" + strings.Repeat("A", 200_000)
	raw, err := json.Marshal(map[string]any{"status": PackageInert, "reason": reason, "pending": true})
	require.NoError(t, err)

	got, err := decodePackageMeta(raw)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(got.Reason), maxPackageReason+len(truncatedMark))
	assert.True(t, utf8.ValidString(got.Reason))
	assert.True(t, strings.HasPrefix(got.Reason, strings.Repeat("a", maxPackageReason-1)))
	assert.True(t, strings.HasSuffix(got.Reason, truncatedMark), "a cut reason says it was cut")
}

func TestDecodePackageMeta_rejectsGarbage(t *testing.T) {
	_, err := decodePackageMeta([]byte("not json"))
	require.Error(t, err)
	_, err = decodePackageMeta([]byte(`{"path":"x"}`))
	require.Error(t, err, "a response with no status says nothing about the path")
}

func TestDecodeSubmissionPolicy(t *testing.T) {
	cases := map[string]string{
		`"inert"`:          "inert",
		`"permissionless"`: "permissionless",
		// A chain predating the policy has no such param.
		``:     "",
		`null`: "",
	}
	for raw, want := range cases {
		got, err := decodeSubmissionPolicy([]byte(raw))
		require.NoError(t, err, "raw=%q", raw)
		assert.Equal(t, want, got, "raw=%q", raw)
	}
	_, err := decodeSubmissionPolicy([]byte(`["inert"]`))
	require.Error(t, err)
}

func TestDecodeAddressList(t *testing.T) {
	got, err := decodeAddressList([]byte(`["g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m","g1manfred47kzduec920z88wfr64ylksmdcedlf5"]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"g1aeddlftlfk27ret5rf750d7w5dume3kcsm8r8m", "g1manfred47kzduec920z88wfr64ylksmdcedlf5"}, got)

	for _, raw := range []string{``, `null`, `[]`} {
		got, err := decodeAddressList([]byte(raw))
		require.NoError(t, err, "raw=%q", raw)
		assert.Empty(t, got, "raw=%q: an unset list switches the gate off", raw)
	}
}
