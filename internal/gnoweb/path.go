package gnoweb

import (
	"fmt"
	"net/url"
	"strings"
)

// PathKind classifies a user-supplied gnoweb URL or gno path.
type PathKind string

const (
	PathNetwork   PathKind = "network"
	PathPackage   PathKind = "package"
	PathRender    PathKind = "render"
	PathFile      PathKind = "file"
	PathSymbol    PathKind = "symbol"
	PathCall      PathKind = "call"
	PathUser      PathKind = "user"
	PathAddress   PathKind = "address"
	PathNamespace PathKind = "namespace"
)

// Path is the chain target encoded in a gnoweb URL or gno path. PkgPath keeps
// the chain-native package path (gno.land/r/...), while RenderPath/File/Symbol
// carry the narrower target when the URL names one.
type Path struct {
	Raw        string   `json:"raw"`
	Domain     string   `json:"domain,omitempty"`
	PkgPath    string   `json:"pkg_path,omitempty"`
	RenderPath string   `json:"render_path,omitempty"`
	File       string   `json:"file,omitempty"`
	Symbol     string   `json:"symbol,omitempty"`
	Address    string   `json:"address,omitempty"`
	User       string   `json:"user,omitempty"`
	Args       []string `json:"args,omitempty"`
	Kind       PathKind `json:"kind"`
}

// ParsePath parses the path/address portion of a gnoweb URL or plain gno path.
// It accepts common paste forms such as:
//
//   - https://gno.land/r/gnoland/blog
//   - https://gno.land/r/gnoland/blog:p/monthly-dev-17
//   - https://gno.land/r/gnoland/blog$source&file=admin.gno
//   - gno.land/r/demo/counter.Increment()
func ParsePath(input string) (Path, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Path{}, fmt.Errorf("empty gnoweb path")
	}

	s := raw
	if u, err := url.Parse(raw); err == nil && u.Scheme != "" && u.Host != "" {
		s = u.Host + u.EscapedPath()
		if u.RawQuery != "" {
			s += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			s += "#" + u.Fragment
		}
	}
	s = strings.TrimRight(s, "/")

	p := Path{Raw: raw}
	if fragBase, frag, ok := strings.Cut(s, "#"); ok {
		s = fragBase
		if strings.HasPrefix(frag, "func-") {
			p.Symbol = strings.TrimPrefix(frag, "func-")
		}
	}

	if strings.HasPrefix(s, "g1") && !strings.ContainsAny(s, "/.") {
		p.Kind = PathAddress
		p.Address = s
		return p, nil
	}

	domain, rest, ok := strings.Cut(s, "/")
	if !ok {
		p.Domain = s
		p.Kind = PathNetwork
		return p, nil
	}
	p.Domain = domain
	rest = "/" + rest
	if strings.HasPrefix(rest, "/r/") || strings.HasPrefix(rest, "/p/") {
		p.Domain = "gno.land"
	}

	if user, ok := strings.CutPrefix(rest, "/u/"); ok {
		p.Kind = PathUser
		p.User = strings.Trim(user, "/")
		return p, nil
	}

	if base, mod, ok := cutGnowebModifier(rest); ok {
		rest = base
		p.RenderPath = "$" + mod.name
		p.File = mod.params.Get("file")
	}

	if base, renderPath, ok := strings.Cut(rest, ":"); ok {
		rest = base
		p.RenderPath = strings.Trim(renderPath, "/")
	}

	if callBase, argsPart, ok := strings.Cut(rest, "("); ok {
		pkgPath, symbol := splitSymbol(p.Domain, callBase)
		if symbol == "" {
			return Path{}, fmt.Errorf("call expression requires a function name")
		}
		args, err := parseCallArgs("(" + argsPart)
		if err != nil {
			return Path{}, err
		}
		p.PkgPath, p.Symbol, p.Args, p.Kind = pkgPath, symbol, args, PathCall
		return p, nil
	}

	if pkgPath, symbol := splitSymbol(p.Domain, rest); symbol != "" {
		p.PkgPath, p.Symbol = pkgPath, firstNonEmpty(p.Symbol, symbol)
		if p.RenderPath != "" {
			p.Kind = PathRender
		} else {
			p.Kind = PathSymbol
		}
		return p, nil
	}

	if lastSlash := strings.LastIndex(rest, "/"); lastSlash > 0 {
		last := rest[lastSlash+1:]
		if isSourceFile(last) {
			p.PkgPath = p.Domain + rest[:lastSlash]
			p.File = last
			p.Kind = PathFile
			return p, nil
		}
	}

	p.PkgPath = p.Domain + rest
	if p.RenderPath != "" {
		p.Kind = PathRender
		return p, nil
	}
	if packageDepth(rest) <= 2 {
		p.Kind = PathNamespace
	} else {
		p.Kind = PathPackage
	}
	return p, nil
}

// BaseURL returns the scheme+host portion of a gnoweb URL, without a trailing
// slash. It is the value profiles should persist as gnoweb-url.
func BaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("gnoweb URL must be absolute")
	}
	return u.Scheme + "://" + u.Host, nil
}

type gnowebModifier struct {
	name   string
	params url.Values
}

func cutGnowebModifier(rest string) (base string, mod gnowebModifier, ok bool) {
	i := strings.Index(rest, "$")
	if i < 0 {
		return "", gnowebModifier{}, false
	}
	base, tail := rest[:i], rest[i+1:]
	name, query, hasQuery := strings.Cut(tail, "?")
	if !hasQuery {
		name, query, _ = strings.Cut(tail, "&")
	}
	params, _ := url.ParseQuery(query)
	return base, gnowebModifier{name: name, params: params}, true
}

func splitSymbol(domain, rest string) (pkgPath, symbol string) {
	lastSlash := strings.LastIndex(rest, "/")
	if lastSlash < 0 {
		return "", ""
	}
	lastSegment := rest[lastSlash+1:]
	if isSourceFile(lastSegment) {
		return "", ""
	}
	dot := strings.Index(lastSegment, ".")
	if dot <= 0 {
		return "", ""
	}
	fullDot := lastSlash + 1 + dot
	return domain + rest[:fullDot], rest[fullDot+1:]
}

func isSourceFile(s string) bool {
	for _, ext := range []string{".gno", ".toml", ".md", ".txt", ".json"} {
		if strings.HasSuffix(s, ext) {
			return true
		}
	}
	return false
}

func packageDepth(rest string) int {
	depth := 0
	for _, part := range strings.Split(strings.Trim(rest, "/"), "/") {
		if part != "" {
			depth++
		}
	}
	return depth
}

func parseCallArgs(s string) ([]string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return nil, fmt.Errorf("invalid call syntax: %s", s)
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	if inner == "" {
		return nil, nil
	}

	var args []string
	var cur strings.Builder
	var quote byte
	hadQuote := false
	for i := 0; i < len(inner); i++ {
		ch := inner[i]
		if ch == '\\' && quote != 0 && i+1 < len(inner) {
			cur.WriteByte(ch)
			i++
			cur.WriteByte(inner[i])
			continue
		}
		if (ch == '"' || ch == '\'') && quote == 0 {
			quote = ch
			hadQuote = true
			continue
		}
		if ch == quote {
			quote = 0
			continue
		}
		if ch == ',' && quote == 0 {
			args = append(args, strings.TrimSpace(cur.String()))
			cur.Reset()
			hadQuote = false
			continue
		}
		cur.WriteByte(ch)
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated string in arguments")
	}
	last := strings.TrimSpace(cur.String())
	if last != "" || hadQuote {
		args = append(args, last)
	}
	return args, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
