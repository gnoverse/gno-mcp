package indexer

import (
	"context"
	"fmt"
	"time"
)

// Fake is an in-memory Client implementation for unit tests.
// Not safe for concurrent use.
type Fake struct {
	lists      map[string][]Realm   // key: filter signature
	histories  map[string][]TxEvent // key: realm
	activities map[string][]TxEvent // key: realm
}

func NewFake() *Fake {
	return &Fake{
		lists:      map[string][]Realm{},
		histories:  map[string][]TxEvent{},
		activities: map[string][]TxEvent{},
	}
}

func filterKey(f ListFilter) string {
	return f.Namespace + "|" + f.Tag + "|" + f.Category
}

func (f *Fake) SetList(filter ListFilter, realms []Realm)  { f.lists[filterKey(filter)] = realms }
func (f *Fake) SetHistory(realm string, events []TxEvent)  { f.histories[realm] = events }
func (f *Fake) SetActivity(realm string, events []TxEvent) { f.activities[realm] = events }

func (f *Fake) List(_ context.Context, filter ListFilter) ([]Realm, error) {
	v, ok := f.lists[filterKey(filter)]
	if !ok {
		return nil, fmt.Errorf("fake: no list for filter %+v", filter)
	}
	return v, nil
}

func (f *Fake) History(_ context.Context, realm string) ([]TxEvent, error) {
	v, ok := f.histories[realm]
	if !ok {
		return nil, fmt.Errorf("fake: no history for %s", realm)
	}
	return v, nil
}

func (f *Fake) Activity(_ context.Context, realm string, since, until *time.Time) ([]TxEvent, error) {
	v, ok := f.activities[realm]
	if !ok {
		return nil, fmt.Errorf("fake: no activity for %s", realm)
	}
	var out []TxEvent
	for _, e := range v {
		if since != nil && e.Time.Before(*since) {
			continue
		}
		if until != nil && e.Time.After(*until) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

var _ Client = (*Fake)(nil)
