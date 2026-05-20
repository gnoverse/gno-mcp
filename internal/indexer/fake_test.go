package indexer

import (
	"context"
	"testing"
	"time"
)

func TestFake_List(t *testing.T) {
	f := NewFake()
	f.SetList(ListFilter{Tag: "fungible"}, []Realm{
		{Path: "gno.land/r/demo/tokens/grc20", Tags: []string{"fungible", "token"}},
	})
	got, err := f.List(context.Background(), ListFilter{Tag: "fungible"})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 || got[0].Path != "gno.land/r/demo/tokens/grc20" {
		t.Errorf("List = %+v", got)
	}
}

func TestFake_History(t *testing.T) {
	f := NewFake()
	f.SetHistory("gno.land/r/foo", []TxEvent{
		{Hash: "0xabc", Height: 100, Kind: "MsgAddPackage"},
	})
	got, err := f.History(context.Background(), "gno.land/r/foo")
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(got) != 1 || got[0].Hash != "0xabc" {
		t.Errorf("History = %+v", got)
	}
}

func TestFake_Activity_filtersByTime(t *testing.T) {
	f := NewFake()
	now := time.Now()
	f.SetActivity("gno.land/r/x", []TxEvent{
		{Hash: "old", Time: now.Add(-2 * time.Hour)},
		{Hash: "new", Time: now.Add(-30 * time.Minute)},
	})
	since := now.Add(-1 * time.Hour)
	got, err := f.Activity(context.Background(), "gno.land/r/x", &since, nil)
	if err != nil {
		t.Fatalf("Activity: %v", err)
	}
	if len(got) != 1 || got[0].Hash != "new" {
		t.Errorf("expected only 'new' after filter, got %+v", got)
	}
}
