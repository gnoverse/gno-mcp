// Command gnomcp is the Gno Model Context Protocol server.
// It exposes Gno chain read tools to MCP clients via stdio transport.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	idxtools "github.com/gnoverse/gno-mcp/internal/tools/indexer"
	readtools "github.com/gnoverse/gno-mcp/internal/tools/read"
	writetools "github.com/gnoverse/gno-mcp/internal/tools/write"
)

const version = "v0.2.0"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println(version)
			return
		case "audit":
			runAudit(os.Args[2:])
			return
		case "profile":
			runProfile(os.Args[2:])
			return
		}
	}

	fs := flag.NewFlagSet("gnomcp", flag.ExitOnError)
	configPath := fs.String("config", "", "explicit profiles.toml path (overrides discovery); empty uses defaults + ~/.config/gnomcp/profiles.toml + ./profiles.toml")
	auditPath := fs.String("audit-path", defaultAuditPath(), "path to audit log file")
	auditReads := fs.Bool("audit-reads", false, "also audit read-only tool calls")
	sessionsPath := fs.String("sessions-path", defaultSessionsPath(), "path to session storage directory")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("flag parse: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ---- load + validate profiles
	cfg, err := profiles.LoadResolved(resolveSources(*configPath))
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// ---- local-gnodev discovery
	discoveredLocal := discoverLocal(ctx, cfg)

	// ---- open audit log
	if err := os.MkdirAll(filepath.Dir(*auditPath), 0o700); err != nil {
		log.Fatalf("mkdir audit dir: %v", err)
	}
	auditFile, err := os.OpenFile(*auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		log.Fatalf("open audit log: %v", err)
	}
	defer auditFile.Close()
	auditLog := audit.NewLog(auditFile)

	// ---- build server + tool resolvers
	s := server.NewServer(cfg, discoveredLocal)
	chainResolver := buildChainResolver(cfg)
	indexerResolver := buildIndexerResolver(cfg)

	// ---- session manager
	passphrase := os.Getenv("GNOMCP_SESSION_PASSPHRASE")
	sessionMgr := session.NewManager(*sessionsPath, passphrase)
	if err := sessionMgr.Hydrate(ctx, chainResolver); err != nil {
		log.Printf("session hydration warning: %v", err)
	}

	// ---- keystore (agent identity for local and testnet profiles)
	ks := keystore.New(defaultAgentKeysPath(), passphrase)

	// ---- register tools
	readtools.RegisterRender(s, chainResolver)
	readtools.RegisterEval(s, chainResolver)
	readtools.RegisterRead(s, chainResolver)
	readtools.RegisterInspect(s, chainResolver)
	readtools.RegisterConnect(s, &http.Client{Timeout: 10 * time.Second})

	if s.AnyProfileHasIndexer() {
		idxtools.RegisterList(s, indexerResolver)
		idxtools.RegisterHistory(s, indexerResolver)
		idxtools.RegisterActivity(s, indexerResolver)
	}

	writetools.RegisterCall(s, ks, sessionMgr, chainResolver, auditLog)
	writetools.RegisterRun(s, ks, sessionMgr, chainResolver, auditLog)
	writetools.RegisterAuthStatus(s, sessionMgr, chainResolver)
	writetools.RegisterSessionPropose(s, sessionMgr)
	writetools.RegisterSessionRevoke(s, sessionMgr)

	if s.AnyProfileAgentCapable() { // agent-only tools — local (test1) or testnet (generated key)
		writetools.RegisterAddPkg(s, ks, chainResolver, auditLog)
		writetools.RegisterKeyAddress(s, ks)
		writetools.RegisterKeyGenerate(s, ks)
	}

	if s.AnyProfileTestnet() {
		writetools.RegisterFaucetFund(s, ks, chainResolver, &http.Client{Timeout: 30 * time.Second})
	}

	// ---- build MCP SDK server
	instructions := "gnomcp exposes Gno chain operations over MCP. Tools register per-profile capability (see profiles.toml), so the available set varies by config. Typical flows:\n" +
		"- READ: gno_inspect (learn a realm's API) then gno_render / gno_eval / gno_read (read state or source).\n" +
		"- WRITE on testnet: gno_key_generate (once) -> gno_faucet_fund (fund the agent key) -> gno_call / gno_run / gno_addpkg. An unfunded write returns insufficient_funds pointing at gno_faucet_fund.\n" +
		"- WRITE as the user (any chain with a master-address): gno_session_propose -> the user runs the printed gnokey command to authorize -> retry the write with identity=session. gno_auth_status / gno_session_revoke inspect and revoke sessions.\n" +
		"- New chain: gno_connect <gnoweb URL> -> run the printed `gnomcp profile add` command -> restart gnomcp.\n" +
		"Always report which identity signed a write (the agent key vs a session) so it is never ambiguous."
	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "gnomcp",
		Version: version,
	}, &mcpsdk.ServerOptions{
		Instructions: instructions,
	})

	for _, t := range s.Registry().All() {
		inputSchema, err := mapToSchema(t.InputSchema)
		if err != nil {
			log.Fatalf("build schema for tool %q: %v", t.Name, err)
		}
		mcpServer.AddTool(&mcpsdk.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: inputSchema,
			Annotations: toSDKAnnotations(t.Annotations),
		}, makeHandler(t, s, auditLog, *auditReads))
	}

	// ---- run
	if err := mcpServer.Run(ctx, &mcpsdk.StdioTransport{}); err != nil {
		log.Printf("server exited: %v", err)
	}
}

// ---- resolver builders

func buildChainResolver(cfg *profiles.Config) chain.Resolver {
	clients := make(map[string]*chain.Real, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		c, err := chain.NewReal(p.RPCURL, p.ChainID)
		if err != nil {
			log.Fatalf("chain client for profile %q: %v", name, err)
		}
		clients[name] = c
	}
	return func(profile string) chain.Client {
		return clients[profile]
	}
}

func buildIndexerResolver(cfg *profiles.Config) indexer.Resolver {
	clients := make(map[string]*indexer.GraphQL, len(cfg.Profiles))
	for name, p := range cfg.Profiles {
		if p.TxIndexerURL != "" {
			clients[name] = indexer.NewGraphQL(p.TxIndexerURL)
		}
	}
	return func(profile string) indexer.Client {
		c, ok := clients[profile]
		if !ok {
			return nil
		}
		return c
	}
}

// ---- MCP handler adapter

func makeHandler(t *server.Tool, s *server.Server, auditLog *audit.Log, auditReads bool) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		// unmarshal raw arguments to map
		var args map[string]any
		if req.Params.Arguments != nil {
			if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("invalid arguments: %v", err)}},
					IsError: true,
				}, nil
			}
		}
		if args == nil {
			args = map[string]any{}
		}

		start := time.Now()
		res, callErr := s.Registry().Call(ctx, t.Name, args)
		dur := time.Since(start)

		// determine audit class
		shouldAudit := t.Capability == server.CapWrite || t.Capability == server.CapWritePrep || auditReads

		if shouldAudit {
			entry := audit.Entry{
				Time:        time.Now().UTC(),
				Tool:        t.Name,
				Profile:     argString(args, "profile"),
				ArgsSummary: argsSummary(args),
				Duration:    dur.Milliseconds(),
			}
			if callErr != nil {
				entry.Result = "tool_err"
			} else {
				entry.Result = "ok"
			}
			_ = auditLog.Append(entry) // audit errors are non-fatal
		}

		if callErr != nil {
			return toolErrorResult(callErr), nil
		}

		return formatResult(res, t.OutputKind), nil
	}
}

// formatResult converts a server.Result to a *mcp.CallToolResult.
func formatResult(res server.Result, kind server.OutputKind) *mcpsdk.CallToolResult {
	var out *mcpsdk.CallToolResult
	switch kind {
	case server.OutputResource:
		mime := res.ResourceMIME
		if mime == "" {
			mime = "text/markdown"
		}
		out = &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.EmbeddedResource{
					Resource: &mcpsdk.ResourceContents{
						URI:      res.ResourceURI,
						MIMEType: mime,
						Text:     res.ResourceBody,
					},
				},
			},
		}
	default: // OutputText
		out = &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: res.Text}},
		}
	}
	if len(res.StructuredContent) > 0 {
		out.StructuredContent = res.StructuredContent
	}
	return out
}

// ---- schema conversion

// mapToSchema converts a map[string]any JSON Schema fragment to a *jsonschema.Schema
// by round-tripping through JSON. Server.AddTool requires a *jsonschema.Schema.
func mapToSchema(m map[string]any) (*jsonschema.Schema, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}
	var s jsonschema.Schema
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("unmarshal schema: %w", err)
	}
	return &s, nil
}

// ---- audit subcommand

func runAudit(args []string) {
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "usage: gnomcp audit <tail|grep <pattern>>\n")
		os.Exit(1)
	}
	switch args[0] {
	case "tail":
		tailAudit()
	case "grep":
		if len(args) < 2 {
			fmt.Fprintf(os.Stderr, "usage: gnomcp audit grep <pattern>\n")
			os.Exit(1)
		}
		grepAudit(args[1])
	default:
		fmt.Fprintf(os.Stderr, "unknown audit subcommand %q\n", args[0])
		os.Exit(1)
	}
}

func tailAudit() {
	path := defaultAuditPath()
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("open audit log: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fmt.Println(sc.Text())
	}
	if err := sc.Err(); err != nil {
		log.Fatalf("read audit log: %v", err)
	}
}

func grepAudit(pattern string) {
	path := defaultAuditPath()
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("open audit log: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, pattern) {
			fmt.Println(line)
		}
	}
	if err := sc.Err(); err != nil {
		log.Fatalf("read audit log: %v", err)
	}
}

// ---- discovery

func discoverLocal(ctx context.Context, cfg *profiles.Config) string {
	for name, p := range cfg.Profiles {
		ok, err := profiles.DiscoverLocal(ctx, p, 2*time.Second)
		if err != nil {
			continue
		}
		if ok {
			return name
		}
	}
	return ""
}

// ---- path defaults

// resolveSources builds the config precedence chain. explicit comes from the
// -config flag or GNOMCP_CONFIG env (flag wins).
func resolveSources(explicit string) profiles.Sources {
	if explicit == "" {
		explicit = os.Getenv("GNOMCP_CONFIG")
	}
	var global string
	if home, err := os.UserHomeDir(); err == nil {
		global = filepath.Join(home, ".config", "gnomcp", "profiles.toml")
	}
	return profiles.Sources{
		GlobalPath:   global,
		ProjectPath:  "profiles.toml",
		ExplicitPath: explicit,
	}
}

func defaultAuditPath() string {
	if v := os.Getenv("GNOMCP_AUDIT_PATH"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "gnomcp", "audit.log")
	}
	return "audit.log"
}

func defaultSessionsPath() string {
	if v := os.Getenv("GNOMCP_SESSIONS_PATH"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "gnomcp", "sessions")
	}
	return "./sessions"
}

func defaultAgentKeysPath() string {
	if v := os.Getenv("GNOMCP_AGENT_KEYS_PATH"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "gnomcp", "agent-keys")
	}
	return "./agent-keys"
}

// ---- helpers

// toSDKAnnotations maps server.Annotations to the MCP SDK ToolAnnotations.
// All four hints are set explicitly so SDK defaults (DestructiveHint=true,
// OpenWorldHint=true) don't misrepresent read-only tools.
func toSDKAnnotations(a server.Annotations) *mcpsdk.ToolAnnotations {
	destructive := a.Destructive
	openWorld := a.OpenWorld
	return &mcpsdk.ToolAnnotations{
		ReadOnlyHint:    a.ReadOnly,
		DestructiveHint: &destructive,
		IdempotentHint:  a.Idempotent,
		OpenWorldHint:   &openWorld,
	}
}

// toolErrorResult builds a CallToolResult for a tool-side error. If err is a
// *server.ToolError its Code, Message, and Extra fields are wired into the
// structured response; otherwise the error string is used as plain text.
func toolErrorResult(err error) *mcpsdk.CallToolResult {
	var te *server.ToolError
	if errors.As(err, &te) {
		sc := make(map[string]any, 1+len(te.Extra))
		for k, v := range te.Extra {
			sc[k] = v
		}
		sc["code"] = te.Code
		return &mcpsdk.CallToolResult{
			Content:           []mcpsdk.Content{&mcpsdk.TextContent{Text: te.Message}},
			StructuredContent: sc,
			IsError:           true,
		}
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: err.Error()}},
		IsError: true,
	}
}

func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

func argsSummary(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	b, err := json.Marshal(args)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
