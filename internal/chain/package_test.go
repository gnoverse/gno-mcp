package chain

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPackageFiles_assemblesSortedByName(t *testing.T) {
	f := NewFake()
	f.SetListing("gno.land/r/demo/foo", []string{"z.gno", "a.gno"})
	f.SetFile("gno.land/r/demo/foo", "z.gno", "package foo // z")
	f.SetFile("gno.land/r/demo/foo", "a.gno", "package foo // a")

	files, err := ReadPackageFiles(context.Background(), f, "gno.land/r/demo/foo")
	require.NoError(t, err)
	require.Len(t, files, 2)
	assert.Equal(t, "a.gno", files[0].Name)
	assert.Equal(t, "package foo // a", files[0].Body)
	assert.Equal(t, "z.gno", files[1].Name)
	assert.Equal(t, "package foo // z", files[1].Body)
}

func TestReadPackageFiles_propagatesListError(t *testing.T) {
	f := NewFake() // no listing seeded
	_, err := ReadPackageFiles(context.Background(), f, "gno.land/r/demo/missing")
	require.Error(t, err)
}
