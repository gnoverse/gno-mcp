// Package gnosrc renders structural views of Gno package source: a
// navigation outline (signatures + docs, bodies elided) and verbatim
// extraction of named declarations with a best-effort dependency header.
//
// Analysis is purely syntactic (go/parser + go/ast); there is deliberately no
// type checking — resolving method calls or interface dispatch would need a
// Gno-aware importer and still be incomplete. Dependency lists are therefore
// navigation hints, never audit evidence, and incompleteness is flagged
// inline rather than hidden.
package gnosrc

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

// File is one source file of a package as fetched from the chain.
type File struct {
	Name string
	Body string
}

// parsedFile pairs a File with its parse outcome. Non-.gno files and files
// that fail to parse keep a nil syntax tree; err carries the parse failure so
// callers can surface it instead of silently dropping the file.
type parsedFile struct {
	name  string
	body  string
	isGno bool
	syn   *ast.File
	err   error
}

// parseAll parses every .gno file against one shared FileSet (offsets in the
// returned trees map back into each file's body for verbatim slicing).
func parseAll(fset *token.FileSet, files []File) []parsedFile {
	out := make([]parsedFile, 0, len(files))
	for _, f := range files {
		pf := parsedFile{name: f.Name, body: f.Body, isGno: strings.HasSuffix(f.Name, ".gno")}
		if pf.isGno {
			// NB: parser object resolution (ast.Ident.Obj) is load-bearing for
			// deps.go's local-vs-package discrimination — do not add
			// parser.SkipObjectResolution. TestExtractSymbols_localShadowNotADep
			// is the safety net.
			pf.syn, pf.err = parser.ParseFile(fset, f.Name, f.Body, parser.ParseComments)
			if pf.err != nil {
				pf.syn = nil
			}
		}
		out = append(out, pf)
	}
	return out
}

// printNode renders an AST node with go/printer against the originating
// FileSet. Comments attached inside the node are dropped (go/printer prints
// bare nodes without comment maps) — callers print doc comments themselves.
func printNode(fset *token.FileSet, n any) string {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, n); err != nil {
		return "(unprintable: " + err.Error() + ")"
	}
	return buf.String()
}

// renderDoc re-renders a doc comment group as //-prefixed lines.
func renderDoc(d *ast.CommentGroup) string {
	if d == nil {
		return ""
	}
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(d.Text(), "\n"), "\n") {
		if line == "" {
			b.WriteString("//\n")
			continue
		}
		b.WriteString("// " + line + "\n")
	}
	return b.String()
}

// elideValues returns a GenDecl holding a copy of spec with initializer
// expressions replaced by an ellipsis marker, for outline rendering.
func elideValues(tok token.Token, spec *ast.ValueSpec) *ast.GenDecl {
	c := *spec
	c.Doc = nil
	c.Comment = nil
	if len(c.Values) > 0 {
		c.Values = []ast.Expr{ast.NewIdent("…")}
	}
	return &ast.GenDecl{Tok: tok, Specs: []ast.Spec{&c}}
}

// signatureOf returns a copy of fd with body and doc stripped, for rendering
// the bare signature.
func signatureOf(fd *ast.FuncDecl) *ast.FuncDecl {
	c := *fd
	c.Body = nil
	c.Doc = nil
	return &c
}
