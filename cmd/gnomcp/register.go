package main

import (
	"fmt"
	"net/http"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	admintools "github.com/gnoverse/gno-mcp/internal/tools/admin"
	idxtools "github.com/gnoverse/gno-mcp/internal/tools/indexer"
	readtools "github.com/gnoverse/gno-mcp/internal/tools/read"
	writetools "github.com/gnoverse/gno-mcp/internal/tools/write"
)

// toolDeps bundles everything tool registration needs, so the whole set can
// be re-registered (schemas regenerated, gates re-evaluated) after a dynamic
// profile add.
type toolDeps struct {
	srv             *server.Server
	chainResolver   chain.Resolver
	indexerResolver indexer.Resolver
	sessionMgr      *session.Manager
	keystore        *keystore.Keystore
	auditLog        *audit.Log
	connectClient   *http.Client // gnoweb fetches (gno_connect, gno_profile_add discovery)
	faucetClient    *http.Client // faucet-service calls (gno_faucet_fund)
	verifyChainID   admintools.ChainIDVerifier
	onProfileAdded  func() error
}

// registerAllTools (re-)registers every tool whose gate currently holds.
// Registry.Add replaces same-name entries, so re-running this after a dynamic
// profile add regenerates the profile enums and summons newly gated tools.
// Profiles are add-only, so gates only flip false->true and the tool set only
// ever grows.
func registerAllTools(d *toolDeps) {
	s := d.srv

	readtools.RegisterRender(s, d.chainResolver)
	readtools.RegisterEval(s, d.chainResolver)
	readtools.RegisterRead(s, d.chainResolver)
	readtools.RegisterPackages(s, d.chainResolver)
	readtools.RegisterAccount(s, d.chainResolver)
	readtools.RegisterStatus(s, d.chainResolver, d.connectClient)
	readtools.RegisterConnect(s, d.connectClient)

	if s.AnyProfileHasIndexer() {
		idxtools.RegisterList(s, d.indexerResolver)
		idxtools.RegisterHistory(s, d.indexerResolver)
		idxtools.RegisterActivity(s, d.indexerResolver)
	}

	writetools.RegisterCall(s, d.keystore, d.sessionMgr, d.chainResolver, d.auditLog)
	writetools.RegisterRun(s, d.keystore, d.sessionMgr, d.chainResolver, d.auditLog)
	writetools.RegisterAuthStatus(s, d.sessionMgr, d.chainResolver)
	writetools.RegisterSessionPropose(s, d.sessionMgr)
	writetools.RegisterSessionRevoke(s, d.sessionMgr)

	// Agent-only tools — every allowed chain (local dev or testnet) has an
	// agent key path (test1 or generated key), so these are unconditional.
	writetools.RegisterAddPkg(s, d.keystore, d.chainResolver, d.auditLog)
	writetools.RegisterKeySend(s, d.keystore, d.chainResolver, d.auditLog)
	writetools.RegisterKeyAddress(s, d.keystore)
	writetools.RegisterKeyList(s, d.keystore)
	writetools.RegisterKeyGenerate(s, d.keystore)
	writetools.RegisterKeyDelete(s, d.keystore, d.chainResolver)

	if s.AnyProfileTestnet() {
		writetools.RegisterFaucetFund(s, d.keystore, d.chainResolver, d.faucetClient)
	}

	admintools.RegisterProfileAdd(s, d.connectClient, d.verifyChainID, d.onProfileAdded)
}

// publishTools pushes the registry's current tool set to the MCP SDK server.
// The SDK's AddTool replaces same-name tools and notifies connected sessions
// (tools/list_changed), so this is safe to call while sessions are live.
func publishTools(m *mcpsdk.Server, s *server.Server, auditLog *audit.Log, auditReads bool) error {
	for _, t := range s.Registry().All() {
		inputSchema, err := mapToSchema(t.InputSchema)
		if err != nil {
			return fmt.Errorf("build schema for tool %q: %w", t.Name, err)
		}
		m.AddTool(&mcpsdk.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: inputSchema,
			Annotations: toSDKAnnotations(t.Annotations),
		}, makeHandler(t, s, auditLog, auditReads))
	}
	return nil
}
