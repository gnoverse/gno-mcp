package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFake_List(t *testing.T) {
	f := NewFake()
	f.SetList(ListFilter{Tag: "fungible"}, []Realm{
		{Path: "gno.land/r/demo/tokens/grc20", Tags: []string{"fungible", "token"}},
	})
	got, err := f.List(context.Background(), ListFilter{Tag: "fungible"})
	require.NoError(t, err, "List")
	require.Len(t, got, 1)
	assert.Equal(t, "gno.land/r/demo/tokens/grc20", got[0].Path)
}

func TestFake_History(t *testing.T) {
	f := NewFake()
	f.SetHistory("gno.land/r/foo", []TxEvent{
		{Hash: "0xabc", Height: 100, Kind: "MsgAddPackage"},
	})
	got, err := f.History(context.Background(), "gno.land/r/foo")
	require.NoError(t, err, "History")
	require.Len(t, got, 1)
	assert.Equal(t, "0xabc", got[0].Hash)
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
	require.NoError(t, err, "Activity")
	require.Len(t, got, 1, "expected only 'new' after filter, got %+v", got)
	assert.Equal(t, "new", got[0].Hash)
}

func TestFake_Activity_filtersByUntil(t *testing.T) {
	f := NewFake()
	now := time.Now()
	f.SetActivity("gno.land/r/x", []TxEvent{
		{Hash: "old", Time: now.Add(-2 * time.Hour)},
		{Hash: "new", Time: now.Add(-30 * time.Minute)},
	})
	until := now.Add(-1 * time.Hour)
	got, err := f.Activity(context.Background(), "gno.land/r/x", nil, &until)
	require.NoError(t, err, "Activity")
	require.Len(t, got, 1, "expected only 'old' before until, got %+v", got)
	assert.Equal(t, "old", got[0].Hash)
}

func TestFake_Activity_unboundedReturnsAll(t *testing.T) {
	f := NewFake()
	now := time.Now()
	f.SetActivity("gno.land/r/x", []TxEvent{
		{Hash: "a", Time: now.Add(-2 * time.Hour)},
		{Hash: "b", Time: now.Add(-30 * time.Minute)},
	})
	got, err := f.Activity(context.Background(), "gno.land/r/x", nil, nil)
	require.NoError(t, err, "Activity")
	assert.Len(t, got, 2, "expected both events with nil/nil bounds, got %+v", got)
}
