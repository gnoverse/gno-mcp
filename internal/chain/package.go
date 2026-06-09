package chain

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/gnolang/gno/tm2/pkg/std"
)

// ReadPackageFiles fetches every file of pkgPath (name + body) for
// whole-package reads, sorted by name for deterministic output. Built on the
// Client's ListFiles + File so it works for any implementation (Real, Fake).
//
// Cost: 1 + N sequential queries and it drops ctx, matching the underlying
// ListFiles/File limitation (gnoclient is not ctx-aware).
func ReadPackageFiles(ctx context.Context, c Client, pkgPath string) ([]*std.MemFile, error) {
	names, err := c.ListFiles(ctx, pkgPath)
	if err != nil {
		return nil, err
	}
	files := make([]*std.MemFile, 0, len(names))
	for _, name := range names {
		body, err := c.File(ctx, pkgPath, name)
		if err != nil {
			return nil, fmt.Errorf("read %s/%s: %w", pkgPath, name, err)
		}
		files = append(files, &std.MemFile{Name: name, Body: body})
	}
	slices.SortFunc(files, func(a, b *std.MemFile) int { return strings.Compare(a.Name, b.Name) })
	return files, nil
}
