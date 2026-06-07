// Package gnoweb discovers a gno chain's connection info from a gnoweb page's
// gnoconnect:* meta-tags. This is an input convenience for creating profiles;
// the profile (config) remains the source of truth.
package gnoweb

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// Conn is the connection info advertised by a gnoweb deployment.
type Conn struct {
	RPC     string
	ChainID string
}

var (
	// metaRE matches a single <meta ...> element; attrRE extracts its quoted
	// attributes (single or double quote). Parsing per-tag makes discovery
	// independent of attribute order (HTML does not guarantee name-before-content).
	metaRE = regexp.MustCompile(`(?i)<meta\b[^>]*>`)
	attrRE = regexp.MustCompile(`(?i)([a-z][a-z0-9:_-]*)\s*=\s*["']([^"']*)["']`)
)

// Discover fetches url and extracts the gnoconnect:rpc and gnoconnect:chainid
// meta-tags (attribute-order independent). Returns an error if either is absent.
func Discover(client *http.Client, url string) (Conn, error) {
	resp, err := client.Get(url)
	if err != nil {
		return Conn{}, fmt.Errorf("fetch gnoweb %q: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Conn{}, fmt.Errorf("fetch gnoweb %q: status %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		return Conn{}, fmt.Errorf("read gnoweb %q: %w", url, err)
	}

	var conn Conn
	for _, tag := range metaRE.FindAll(body, -1) {
		attrs := map[string]string{}
		for _, m := range attrRE.FindAllSubmatch(tag, -1) {
			attrs[strings.ToLower(string(m[1]))] = string(m[2])
		}
		switch attrs["name"] {
		case "gnoconnect:rpc":
			conn.RPC = attrs["content"]
		case "gnoconnect:chainid":
			conn.ChainID = attrs["content"]
		}
	}
	if conn.RPC == "" || conn.ChainID == "" {
		return Conn{}, fmt.Errorf("gnoweb %q: no gnoconnect:rpc/chainid meta-tags (is this a gnoweb host?)", url)
	}
	return conn, nil
}
