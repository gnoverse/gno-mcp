package gnosrc

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"

	"golang.org/x/tools/txtar"
)

// Extraction is the result of ExtractSymbols. Body holds the matched
// declarations as a txtar archive (one entry per file that matched, verbatim
// source bytes). Misses and parse failures are reported, never swallowed:
// Missing lists requested names that matched nothing, Available the names
// that could have been requested, Errors any per-file parse failures.
type Extraction struct {
	Body      string
	Found     []string
	Missing   []string
	Available []string
	Errors    []string
}

// symbolRef locates one addressable declaration. For names declared in a
// grouped const/var/type block the decl is the whole GenDecl — verbatim
// slicing never splits a group, so siblings come along.
type symbolRef struct {
	file *parsedFile
	decl ast.Decl
	doc  *ast.CommentGroup
	fn   *ast.FuncDecl // non-nil when the symbol is a function or method
}

// ExtractSymbols returns the verbatim source of the named declarations.
// Names address top-level declarations ("Transfer", "count") and methods
// ("Counter.Inc"). Function entries carry a best-effort dependency header
// (same-package references + imports used); unresolved method calls are
// counted and flagged inline, because a clean-looking header over an
// incomplete analysis would hide exactly what an auditor needs to know.
func ExtractSymbols(files []File, names []string) Extraction {
	fset := token.NewFileSet()
	parsed := parseAll(fset, files)

	var x Extraction
	for i := range parsed {
		if parsed[i].err != nil {
			x.Errors = append(x.Errors, fmt.Sprintf("%s: %v", parsed[i].name, parsed[i].err))
		}
	}

	table := symbolTable(parsed)
	x.Available = make([]string, 0, len(table))
	for name := range table {
		x.Available = append(x.Available, name)
	}
	slices.Sort(x.Available)

	pkg := packageScope(parsed)

	// Group matches per file in source order so entries read like the file.
	// Two names can resolve to the same grouped decl (var/const/type blocks);
	// both report as Found but the block is emitted once.
	blocks := make(map[*parsedFile][]symbolRef)
	seenName := make(map[string]bool)
	seenDecl := make(map[ast.Decl]bool)
	for _, name := range names {
		if seenName[name] {
			continue
		}
		seenName[name] = true
		ref, ok := table[name]
		if !ok {
			x.Missing = append(x.Missing, name)
			continue
		}
		x.Found = append(x.Found, name)
		if seenDecl[ref.decl] {
			continue
		}
		seenDecl[ref.decl] = true
		blocks[ref.file] = append(blocks[ref.file], ref)
	}

	ar := &txtar.Archive{}
	for i := range parsed {
		refs := blocks[&parsed[i]]
		if len(refs) == 0 {
			continue
		}
		slices.SortFunc(refs, func(a, b symbolRef) int { return cmp.Compare(a.decl.Pos(), b.decl.Pos()) })
		ar.Files = append(ar.Files, txtar.File{
			Name: parsed[i].name,
			Data: []byte(renderRefs(fset, refs, pkg)),
		})
	}
	if len(ar.Files) > 0 {
		x.Body = string(txtar.Format(ar))
	}
	return x
}

func renderRefs(fset *token.FileSet, refs []symbolRef, pkg *packageNames) string {
	var b strings.Builder
	for i, ref := range refs {
		if i > 0 {
			b.WriteString("\n")
		}
		if ref.fn != nil {
			b.WriteString(depHeader(fset, ref.fn, ref.file, pkg))
		}
		b.WriteString(verbatim(fset, ref) + "\n")
	}
	return b.String()
}

// verbatim slices the declaration (including its doc comment) out of the
// original file body — exact source bytes, never re-rendered. Within a
// requested declaration nothing is ever elided; the body is the evidence.
func verbatim(fset *token.FileSet, ref symbolRef) string {
	start := ref.decl.Pos()
	if ref.doc != nil {
		start = ref.doc.Pos()
	}
	from := fset.Position(start).Offset
	to := fset.Position(ref.decl.End()).Offset
	return ref.file.body[from:to]
}

// symbolTable indexes every addressable declaration: top-level funcs, vars,
// consts and types by name, methods as "Type.Method".
func symbolTable(parsed []parsedFile) map[string]symbolRef {
	table := make(map[string]symbolRef)
	for i := range parsed {
		pf := &parsed[i]
		if pf.syn == nil {
			continue
		}
		for _, decl := range pf.syn.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				if recv := receiverTypeName(d); recv != "" {
					name = recv + "." + name
				}
				table[name] = symbolRef{file: pf, decl: d, doc: d.Doc, fn: d}
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				for _, spec := range d.Specs {
					for _, id := range specNames(spec) {
						table[id.Name] = symbolRef{file: pf, decl: d, doc: d.Doc}
					}
				}
			}
		}
	}
	return table
}

func specNames(spec ast.Spec) []*ast.Ident {
	switch sp := spec.(type) {
	case *ast.TypeSpec:
		return []*ast.Ident{sp.Name}
	case *ast.ValueSpec:
		return sp.Names
	}
	return nil
}

// receiverTypeName returns the base type name of a method receiver, or ""
// for plain functions and receivers that are not plain (star-)idents.
func receiverTypeName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return ""
	}
	t := d.Recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}
