package indexer

import (
	"context"
	"fmt"
	"time"
)

// Fake is an in-memory Client implementation for unit tests.
// Not safe for concurrent use.
type Fake struct {
	histories  map[string][]TxEvent
	activities map[string][]TxEvent
}

func NewFake() *Fake {
	return &Fake{
		histories:  map[string][]TxEvent{},
		activities: map[string][]TxEvent{},
	}
}

func (f *Fake) SetHistory(realm string, events []TxEvent)  { f.histories[realm] = events }
func (f *Fake) SetActivity(realm string, events []TxEvent) { f.activities[realm] = events }

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
