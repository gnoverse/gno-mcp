// Package gnoweb discovers a gno chain's connection info from a gnoweb page's
// gnoconnect:* meta-tags. This is an input convenience for creating profiles;
// the profile (config) remains the source of truth.
package gnoweb

import (
	"fmt"
	"io"
	"net/http"

	"golang.org/x/net/html"
)

// gnoconnect meta-tag names a gnoweb page advertises its connection info under.
const (
	metaNameRPC     = "gnoconnect:rpc"
	metaNameChainID = "gnoconnect:chainid"
)

// Conn is the connection info advertised by a gnoweb deployment.
type Conn struct {
	RPC     string
	ChainID string
}

// Discover fetches url and extracts the gnoconnect:rpc and gnoconnect:chainid
// meta-tags, reading the document head: tokenizing stops at the first </head>
// or <body>, so tags in the body are ignored. A bare fragment with no head/body
// wrapper is read to EOF (bounded by the 1 MiB cap). Returns an error if either
// tag is absent.
func Discover(client *http.Client, url string) (Conn, error) {
	resp, err := client.Get(url)
	if err != nil {
		return Conn{}, fmt.Errorf("fetch gnoweb %q: %w", url, err)
	}
	defer resp.Body.Close()
	status := resp.StatusCode

	var conn Conn
	z := html.NewTokenizer(io.LimitReader(resp.Body, 1<<20)) // 1 MiB cap
head:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break head // EOF or malformed tail; whatever we parsed stands
		case html.EndTagToken:
			if name, _ := z.TagName(); string(name) == "head" {
				break head
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "body":
				break head // past the head; gnoconnect tags live in <head>
			case "meta":
				// A repeated tag: the last occurrence read wins.
				switch metaName, content := metaAttrs(z); metaName {
				case metaNameRPC:
					conn.RPC = content
				case metaNameChainID:
					conn.ChainID = content
				}
			}
		}
	}
	if conn.RPC == "" || conn.ChainID == "" {
		// Include the HTTP status when it was non-200: helps diagnose a
		// misconfigured URL that returns a non-gnoweb error page.
		if status != http.StatusOK {
			return Conn{}, fmt.Errorf("gnoweb %q: status %d, no %s/%s meta-tags (is this a gnoweb host?)", url, status, metaNameRPC, metaNameChainID)
		}
		return Conn{}, fmt.Errorf("gnoweb %q: no %s/%s meta-tags (is this a gnoweb host?)", url, metaNameRPC, metaNameChainID)
	}
	return conn, nil
}

// metaAttrs reads the name and content attributes of the current <meta> token.
// The tokenizer lower-cases attribute keys, so matching is order-independent.
func metaAttrs(z *html.Tokenizer) (name, content string) {
	for {
		k, v, more := z.TagAttr()
		switch string(k) {
		case "name":
			name = string(v)
		case "content":
			content = string(v)
		}
		if !more {
			return name, content
		}
	}
}
