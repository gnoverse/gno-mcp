package gnosrc

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"golang.org/x/tools/txtar"

	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// Outline renders the navigation view of a package as a txtar archive: one
// entry per file (entry names are the exact file names), exported
// declarations with doc comments and bodies elided, unexported declarations
// as bare signatures, per-file import lists and byte counts. Parse failures
// and non-.gno files are surfaced as size-only entries — never dropped.
//
// The result is server-rendered (not verbatim source), so the whole archive
// is neutralized against untrusted-content envelope forgery. It is a
// navigation aid: names and docs are realm-authored claims, not evidence.
func Outline(files []File) string {
	fset := token.NewFileSet()
	parsed := parseAll(fset, files)
	ar := &txtar.Archive{Files: make([]txtar.File, 0, len(parsed))}
	for _, pf := range parsed {
		ar.Files = append(ar.Files, txtar.File{Name: pf.name, Data: []byte(outlineEntry(fset, pf))})
	}
	return untrusted.Neutralize(string(txtar.Format(ar)))
}

func outlineEntry(fset *token.FileSet, pf parsedFile) string {
	switch {
	case !pf.isGno:
		return fmt.Sprintf("// %d bytes (not Gno source — content omitted; fetch with full=true)\n", len(pf.body))
	case pf.err != nil:
		return fmt.Sprintf("// %d bytes (parse error: %v — fetch with full=true)\n", len(pf.body), pf.err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// %d bytes (outline — bodies elided; fetch source via symbols=[...] or full=true)\n", len(pf.body))
	if paths := importPaths(pf.syn); len(paths) > 0 {
		fmt.Fprintf(&b, "// imports: %s\n", strings.Join(paths, ", "))
	}
	if pf.syn.Doc != nil {
		b.WriteString("\n" + renderDoc(pf.syn.Doc))
		fmt.Fprintf(&b, "package %s\n", pf.syn.Name.Name)
	}

	var unexported []string
	for _, decl := range pf.syn.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() {
				b.WriteString("\n" + renderDoc(d.Doc))
				b.WriteString(printNode(fset, signatureOf(d)) + "\n")
			} else {
				unexported = append(unexported, printNode(fset, signatureOf(d)))
			}
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}
			outlineGenDecl(fset, d, &b, &unexported)
		}
	}

	if len(unexported) > 0 {
		b.WriteString("\n// unexported:\n")
		for _, sig := range unexported {
			for _, line := range strings.Split(strings.TrimRight(sig, "\n"), "\n") {
				b.WriteString("//\t" + line + "\n")
			}
		}
	}
	return b.String()
}

// outlineGenDecl renders each spec of a const/var/type decl: exported specs
// inline with their doc, unexported ones appended to the unexported list.
// Initializer expressions are always elided — values are bodies, not surface.
func outlineGenDecl(fset *token.FileSet, d *ast.GenDecl, b *strings.Builder, unexported *[]string) {
	for _, spec := range d.Specs {
		doc, rendered, exported := outlineSpec(fset, d, spec)
		if rendered == "" {
			continue
		}
		if exported {
			b.WriteString("\n" + doc)
			b.WriteString(rendered + "\n")
		} else {
			*unexported = append(*unexported, rendered)
		}
	}
}

func outlineSpec(fset *token.FileSet, d *ast.GenDecl, spec ast.Spec) (doc, rendered string, exported bool) {
	switch sp := spec.(type) {
	case *ast.TypeSpec:
		c := *sp
		c.Doc = nil
		c.Comment = nil
		doc = specDoc(d, sp.Doc)
		rendered = printNode(fset, &ast.GenDecl{Tok: token.TYPE, Specs: []ast.Spec{&c}})
		exported = sp.Name.IsExported()
	case *ast.ValueSpec:
		doc = specDoc(d, sp.Doc)
		rendered = printNode(fset, elideValues(d.Tok, sp))
		for _, n := range sp.Names {
			exported = exported || n.IsExported()
		}
	}
	return doc, rendered, exported
}

// specDoc prefers the spec's own doc and falls back to the decl doc (the
// usual home of the comment when the decl holds a single spec).
func specDoc(d *ast.GenDecl, specDoc *ast.CommentGroup) string {
	if specDoc != nil {
		return renderDoc(specDoc)
	}
	return renderDoc(d.Doc)
}

// importPaths returns the unquoted import paths of f in source order.
func importPaths(f *ast.File) []string {
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		out = append(out, strings.Trim(imp.Path.Value, `"`))
	}
	return out
}
