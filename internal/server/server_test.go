package server

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer_registersZeroToolsInitially(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {RPCURL: "x", ChainID: "test5"},
	}}
	s := NewServer(cfg, "")
	assert.Equal(t, 0, s.Registry().Count())
}

func TestServer_anyProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"testnet5": {TxIndexerURL: "x"},
		"local":    {},
	}}
	s := NewServer(cfg, "")
	assert.True(t, s.AnyProfileHasIndexer(), "AnyProfileHasIndexer should be true")
}

func TestServer_noProfileHasIndexer(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {},
	}}
	s := NewServer(cfg, "")
	assert.False(t, s.AnyProfileHasIndexer(), "AnyProfileHasIndexer should be false")
}

func TestServer_anyProfileTestnet(t *testing.T) {
	local := &profiles.Config{Profiles: map[string]profiles.Profile{"local": {ChainID: "dev", RPCURL: "x"}}}
	assert.False(t, NewServer(local, "").AnyProfileTestnet(), "local-only -> false")
	tn := &profiles.Config{Profiles: map[string]profiles.Profile{"t": {ChainID: "test5", RPCURL: "x"}}}
	assert.True(t, NewServer(tn, "").AnyProfileTestnet(), "testnet -> true")
}

// ---- Dynamic profiles (in-memory adds; init-time profiles immutable)

func dynServer(t *testing.T) *Server {
	t.Helper()
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local":   {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
		"testnet": {RPCURL: "https://rpc.test11.testnets.gno.land:443", ChainID: "test11"},
	}}
	return NewServer(cfg, "")
}

func TestAddDynamicProfile_addsAndIsVisible(t *testing.T) {
	s := dynServer(t)
	err := s.AddDynamicProfile("test13", profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test-13"})
	require.NoError(t, err)
	p, ok := s.Config().Profiles["test13"]
	require.True(t, ok, "added profile must be visible via Config()")
	assert.Equal(t, "test-13", p.ChainID)
}

func TestAddDynamicProfile_snapshotsAreCopyOnWrite(t *testing.T) {
	s := dynServer(t)
	before := s.Config()
	require.NoError(t, s.AddDynamicProfile("test13", profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test-13"}))
	_, inOld := before.Profiles["test13"]
	assert.False(t, inOld, "a previously captured Config snapshot must not gain the new profile (copy-on-write)")
}

func TestAddDynamicProfile_rejectsInitProfiles(t *testing.T) {
	s := dynServer(t)
	err := s.AddDynamicProfile("testnet", profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test5"})
	require.Error(t, err, "init-time profiles must be immutable")
	assert.ErrorIs(t, err, ErrProfileImmutable)
	assert.Equal(t, "test11", s.Config().Profiles["testnet"].ChainID, "init profile must be unchanged")
}

func TestAddDynamicProfile_rejectsReservedAndInvalidNames(t *testing.T) {
	s := dynServer(t)
	p := profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test5"}

	err := s.AddDynamicProfile("default", p)
	assert.ErrorIs(t, err, ErrProfileReserved, `"default" is reserved`)

	for _, bad := range []string{"", "Foo", "has space", "x;y", "x$(y)"} {
		err := s.AddDynamicProfile(bad, p)
		assert.ErrorIs(t, err, ErrProfileNameInvalid, "name %q must be rejected", bad)
	}
}

func TestAddDynamicProfile_reAddOfDynamicNameReplaces(t *testing.T) {
	s := dynServer(t)
	require.NoError(t, s.AddDynamicProfile("test13", profiles.Profile{RPCURL: "https://rpc.one", ChainID: "test-13"}))
	require.NoError(t, s.AddDynamicProfile("test13", profiles.Profile{RPCURL: "https://rpc.two", ChainID: "test-13"}),
		"re-adding a dynamically added profile must be allowed (agent fixing its own mistake)")
	assert.Equal(t, "https://rpc.two", s.Config().Profiles["test13"].RPCURL)
}

func TestAddDynamicProfile_flipsRegistrationGates(t *testing.T) {
	localOnly := NewServer(&profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {RPCURL: "http://127.0.0.1:26657", ChainID: "dev"},
	}}, "")
	assert.False(t, localOnly.AnyProfileTestnet())
	assert.False(t, localOnly.AnyProfileHasIndexer())

	require.NoError(t, localOnly.AddDynamicProfile("test13", profiles.Profile{
		RPCURL: "https://rpc.example", ChainID: "test-13", TxIndexerURL: "https://idx.example",
	}))
	assert.True(t, localOnly.AnyProfileTestnet(), "adding a testnet profile must flip AnyProfileTestnet")
	assert.True(t, localOnly.AnyProfileHasIndexer(), "adding an indexer-bearing profile must flip AnyProfileHasIndexer")
}

func TestAddDynamicProfile_profileSchemaGrowsButDefaultIsStable(t *testing.T) {
	s := dynServer(t)
	before := s.ProfileSchema()
	require.NoError(t, s.AddDynamicProfile("test13", profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test-13"}))
	after := s.ProfileSchema()
	assert.Contains(t, after.Enum, "test13", "enum must grow with the dynamic profile")
	assert.Equal(t, before.Default, after.Default, "the smart default must not migrate to a dynamic profile")
}

func TestAddDynamicProfile_concurrentAddsAndReads(t *testing.T) {
	s := dynServer(t)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = s.AddDynamicProfile(fmt.Sprintf("dyn%d", i), profiles.Profile{RPCURL: "https://rpc.example", ChainID: "test5"})
			}
		}(i)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = s.Config().Profiles
				_ = s.ProfileSchema()
				_ = s.AnyProfileTestnet()
				_ = s.AnyProfileHasIndexer()
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 4; i++ {
		_, ok := s.Config().Profiles[fmt.Sprintf("dyn%d", i)]
		assert.True(t, ok, "dyn%d must have landed", i)
	}
}

func TestServer_callsRegisteredTool(t *testing.T) {
	cfg := &profiles.Config{Profiles: map[string]profiles.Profile{
		"local": {RPCURL: "x", ChainID: "dev"},
	}}
	s := NewServer(cfg, "")
	s.Registry().Add(&Tool{
		Name: "x", Capability: CapBaseRead,
		Handler: func(ctx context.Context, args map[string]any) (Result, error) {
			return Result{Text: "hi"}, nil
		},
	})
	res, err := s.Registry().Call(context.Background(), "x", nil)
	require.NoError(t, err)
	assert.Equal(t, "hi", res.Text)
}
