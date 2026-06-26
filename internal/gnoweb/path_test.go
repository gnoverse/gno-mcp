package gnoweb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Path
	}{
		{
			name: "package URL",
			in:   "https://gno.land/r/gnoland/blog",
			want: Path{Raw: "https://gno.land/r/gnoland/blog", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", Kind: PathPackage},
		},
		{
			name: "render path URL",
			in:   "https://gno.land/r/gnoland/blog:p/monthly-dev-17",
			want: Path{Raw: "https://gno.land/r/gnoland/blog:p/monthly-dev-17", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", RenderPath: "p/monthly-dev-17", Kind: PathRender},
		},
		{
			name: "testnet host render path URL",
			in:   "https://test13.testnets.gno.land/r/gnoland/blog:p/monthly-dev-17",
			want: Path{Raw: "https://test13.testnets.gno.land/r/gnoland/blog:p/monthly-dev-17", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", RenderPath: "p/monthly-dev-17", Kind: PathRender},
		},
		{
			name: "source file URL",
			in:   "https://gno.land/r/gnoland/blog$source&file=admin.gno",
			want: Path{Raw: "https://gno.land/r/gnoland/blog$source&file=admin.gno", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", RenderPath: "$source", File: "admin.gno", Kind: PathRender},
		},
		{
			name: "file path",
			in:   "gno.land/r/gnoland/blog/admin.gno",
			want: Path{Raw: "gno.land/r/gnoland/blog/admin.gno", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", File: "admin.gno", Kind: PathFile},
		},
		{
			name: "symbol path",
			in:   "gno.land/r/gnoland/blog.ModAddPost",
			want: Path{Raw: "gno.land/r/gnoland/blog.ModAddPost", Domain: "gno.land", PkgPath: "gno.land/r/gnoland/blog", Symbol: "ModAddPost", Kind: PathSymbol},
		},
		{
			name: "call path",
			in:   `gno.land/r/demo/counter.Increment("x", 2)`,
			want: Path{Raw: `gno.land/r/demo/counter.Increment("x", 2)`, Domain: "gno.land", PkgPath: "gno.land/r/demo/counter", Symbol: "Increment", Args: []string{"x", "2"}, Kind: PathCall},
		},
		{
			name: "user URL",
			in:   "https://gno.land/u/moul",
			want: Path{Raw: "https://gno.land/u/moul", Domain: "gno.land", User: "moul", Kind: PathUser},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePath(tt.in)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParsePath_BadCall(t *testing.T) {
	_, err := ParsePath(`gno.land/r/demo/counter.Increment("x"`)
	require.Error(t, err)
}
