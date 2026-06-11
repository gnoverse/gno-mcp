package gnosrc

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strings"
)

// packageNames is the package-level declaration surface used to resolve
// identifier references in dependency analysis: the set of top-level names
// (across all files) plus, per file, the ast.Objects the parser created for
// that file's own top-level declarations (used to tell a same-file
// package-level reference from a shadowing local).
type packageNames struct {
	names   map[string]bool
	topObjs map[*ast.Object]bool
}

func packageScope(parsed []parsedFile) *packageNames {
	pkg := &packageNames{names: make(map[string]bool), topObjs: make(map[*ast.Object]bool)}
	for i := range parsed {
		if parsed[i].syn == nil {
			continue
		}
		for _, decl := range parsed[i].syn.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv != nil {
					continue // methods are addressed via selectors, not bare idents
				}
				pkg.add(d.Name)
			case *ast.GenDecl:
				if d.Tok == token.IMPORT {
					continue
				}
				for _, spec := range d.Specs {
					for _, id := range specNames(spec) {
						pkg.add(id)
					}
				}
			}
		}
	}
	return pkg
}

func (p *packageNames) add(id *ast.Ident) {
	p.names[id.Name] = true
	if id.Obj != nil {
		p.topObjs[id.Obj] = true
	}
}

// isPackageRef reports whether id references a package-level declaration.
// The parser resolves identifiers within their own file: a non-nil Obj that
// is not a known top-level object is a local (param, :=, range var, …); a
// nil Obj is a cross-file reference, package-level iff the name is declared
// anywhere in the package.
func (p *packageNames) isPackageRef(id *ast.Ident) bool {
	if !p.names[id.Name] {
		return false
	}
	if id.Obj != nil {
		return p.topObjs[id.Obj]
	}
	return true
}

// depHeader renders the best-effort dependency comment for a function:
// same-package top-level names its signature and body reference, the import
// paths it actually touches, and a loud incompleteness marker when the body
// contains method calls this syntactic analysis cannot resolve.
func depHeader(fset *token.FileSet, fn *ast.FuncDecl, pf *parsedFile, pkg *packageNames) string {
	deps, imports, unresolved := collectDeps(fn, pf, pkg)

	var b strings.Builder
	if len(deps) > 0 || unresolved > 0 {
		b.WriteString("// deps: " + strings.Join(deps, ", "))
		if unresolved > 0 {
			if len(deps) > 0 {
				b.WriteString(" ")
			}
			fmt.Fprintf(&b, "(+%d unresolved method call(s) — dep list incomplete; read the full file for the complete picture)", unresolved)
		}
		b.WriteString("\n")
	}
	if len(imports) > 0 {
		b.WriteString("// imports: " + strings.Join(imports, ", ") + "\n")
	}
	return b.String()
}

// collectDeps walks fn's receiver, signature, and body. Resolution is
// syntactic: selector roots that name an import count as import usage,
// other selector calls count as unresolved, bare identifiers resolve via
// packageNames. Struct-literal field keys are skipped (they are field names,
// not references — at the cost of missing the rare map literal keyed by a
// package-level const, which the unresolved philosophy tolerates: dep lists
// are hints, not proofs). A method's own receiver base type is excluded —
// requesting Type.Method already names the type.
func collectDeps(fn *ast.FuncDecl, pf *parsedFile, pkg *packageNames) (deps, imports []string, unresolved int) {
	importLocals := importLocalNames(pf.syn)
	depSet := make(map[string]bool)
	importSet := make(map[string]bool)
	recvType := receiverTypeName(fn)

	var visit func(n ast.Node) bool
	walk := func(n ast.Node) {
		if n != nil {
			ast.Inspect(n, visit)
		}
	}
	visit = func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if path, ok := importQualifier(x, importLocals); ok {
				importSet[path] = true
				return false
			}
			walk(x.X) // x.Sel is a field or method name, not an identifier reference
			return false
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if _, isImport := importQualifier(sel, importLocals); !isImport {
					unresolved++
				}
			}
			return true
		case *ast.CompositeLit:
			walk(x.Type)
			for _, elt := range x.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if _, isIdent := kv.Key.(*ast.Ident); isIdent {
						walk(kv.Value)
						continue
					}
				}
				walk(elt)
			}
			return false
		case *ast.Ident:
			if x.Name != recvType && pkg.isPackageRef(x) {
				depSet[x.Name] = true
			}
			return false
		}
		return true
	}
	if fn.Recv != nil {
		walk(fn.Recv)
	}
	walk(fn.Type)
	if fn.Body != nil {
		walk(fn.Body)
	}

	for name := range depSet {
		deps = append(deps, name)
	}
	slices.Sort(deps)
	for path := range importSet {
		imports = append(imports, path)
	}
	slices.Sort(imports)
	return deps, imports, unresolved
}

// importQualifier reports whether sel is an import-qualified reference
// (pkg.Symbol) and returns the import path. A selector root with a non-nil
// Obj is a local variable, never an import.
func importQualifier(sel *ast.SelectorExpr, importLocals map[string]string) (string, bool) {
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Obj != nil {
		return "", false
	}
	path, ok := importLocals[id.Name]
	return path, ok
}

// importLocalNames maps each import's local name (explicit alias or last
// path segment) to its import path. Dot- and blank-imports are skipped.
func importLocalNames(f *ast.File) map[string]string {
	out := make(map[string]string, len(f.Imports))
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		local := path[strings.LastIndex(path, "/")+1:]
		if imp.Name != nil {
			local = imp.Name.Name
		}
		if local == "." || local == "_" {
			continue
		}
		out[local] = path
	}
	return out
}
