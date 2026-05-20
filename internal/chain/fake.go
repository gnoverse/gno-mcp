package chain

import (
	"context"
	"fmt"
)

type Fake struct {
	renders map[string]string // key: realm+"|"+path
	evals   map[string]string // key: realm+"|"+expr
	files   map[string]string // key: realm+"|"+file
	docs    map[string]string // key: realm
}

func NewFake() *Fake {
	return &Fake{
		renders: map[string]string{},
		evals:   map[string]string{},
		files:   map[string]string{},
		docs:    map[string]string{},
	}
}

func (f *Fake) SetRender(realm, path, body string) { f.renders[realm+"|"+path] = body }
func (f *Fake) SetEval(realm, expr, result string) { f.evals[realm+"|"+expr] = result }
func (f *Fake) SetFile(realm, file, body string)   { f.files[realm+"|"+file] = body }
func (f *Fake) SetDoc(realm, doc string)           { f.docs[realm] = doc }

func (f *Fake) Render(_ context.Context, realm, path string) (string, error) {
	v, ok := f.renders[realm+"|"+path]
	if !ok {
		return "", fmt.Errorf("fake: no render for %s|%s", realm, path)
	}
	return v, nil
}

func (f *Fake) Eval(_ context.Context, realm, expr string) (string, error) {
	v, ok := f.evals[realm+"|"+expr]
	if !ok {
		return "", fmt.Errorf("fake: no eval for %s|%s", realm, expr)
	}
	return v, nil
}

func (f *Fake) File(_ context.Context, realm, file string) (string, error) {
	v, ok := f.files[realm+"|"+file]
	if !ok {
		return "", fmt.Errorf("fake: no file for %s|%s", realm, file)
	}
	return v, nil
}

func (f *Fake) Doc(_ context.Context, realm string) (string, error) {
	v, ok := f.docs[realm]
	if !ok {
		return "", fmt.Errorf("fake: no doc for %s", realm)
	}
	return v, nil
}

// Assert Fake satisfies the interface at compile time.
var _ Client = (*Fake)(nil)
