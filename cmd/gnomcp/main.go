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
	"maps"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/gnoverse/gno-mcp/internal/audit"
	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/indexer"
	"github.com/gnoverse/gno-mcp/internal/keystore"
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
	"github.com/gnoverse/gno-mcp/internal/untrusted"
)

// version is overridden at release time via -ldflags "-X main.version=...";
// dev builds report "dev".
var version = "dev"

// maxMCPTextLen is the length, in characters, of the server instructions and
// of each tool description that Claude Code sends the model by default; it
// drops the rest.
const maxMCPTextLen = 2048

// serverInstructions is the MCP initialize-time guidance: cross-tool flows the
// per-tool descriptions can't express (tool descriptions cover one tool each).
// It comes first, so a client that cuts the text keeps it whole.
const serverInstructions = "Indexer and faucet tools exist only for profiles that provide them.\n" +
	"- IDENTITY: keys and mnemonics never enter the conversation; never request key material. The agent key signs agent writes. " +
	"Writing AS the user needs a session the user approves with their own gnokey: gno_session_propose (with no profile master-address, " +
	"pass the user's PUBLIC g1 address as master_address), the user runs the printed command, then retry with identity=session. " +
	"Sessions cover gno_call and gno_run; gno_addpkg is not session-signed.\n" +
	"- DISCOVER: never guess a package path; gno_packages lists a prefix or @namespace. gno_profile_list maps a chain the user names " +
	"(\"on onyx\") to a profile. A gnoweb URL names its chain: resolve it with gno_profile_add (gnoweb_url=...), never read it on the ambient profile.\n" +
	"- READ: gno_read (outline by default; symbols=[...] or full=true for source), gno_render, gno_eval. An outline is navigation, not evidence.\n" +
	"- WRITE (testnet): gno_key_generate once, gno_faucet_fund, then gno_call / gno_run / gno_addpkg; the key arg picks among several agent keys. " +
	"On cla_unsigned, show the user gno_cla_info's agreement URL, then gno_cla_sign once they confirm. A deploy is callable only once " +
	"gno_addpkg reports package_status live. Where deploys park (onyx), a parked path fails with package_parked, and gno_run fails with run_not_allowed.\n" +
	"- RECOVER: tool errors carry their repair; perform it when the user's request already authorizes it, and hand back only the steps " +
	"that need the user (gnokey, a client restart).\n" +
	"Report which identity signed every write.\n"

// buildServerInstructions appends the configured profiles to the static
// guidance so the agent knows which chains it can target without calling a
// tool first. It lists what EXISTS, not what is reachable, sorted by name so
// the text (and the prompt cache) is stable across restarts, and stops at
// maxMCPTextLen with a pointer to gno_profile_list.
func buildServerInstructions(profs map[string]profiles.Profile) string {
	var b strings.Builder
	b.WriteString(serverInstructions)
	b.WriteString("Profiles at startup, what exists, not what is reachable (gno_profile_list is current):\n")

	names := slices.Sorted(maps.Keys(profs))
	for i, name := range names {
		line := profileLine(name, profs[name])
		reserve := 0
		if rest := len(names) - i - 1; rest > 0 {
			reserve = utf8.RuneCountInString(moreProfilesLine(rest))
		}
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(line)+reserve > maxMCPTextLen {
			b.WriteString(moreProfilesLine(len(names) - i))
			break
		}
		b.WriteString(line)
	}
	return b.String()
}

func profileLine(name string, p profiles.Profile) string {
	line := fmt.Sprintf("- %s: %s at %s, %s", name, p.ChainID, p.RPCURL, p.Kind())
	switch {
	case p.IsReadOnly():
		line += " (read tools only)"
	case p.Sunset:
		line += " (sunset: still fully writable, prefer the current testnet)"
	}
	if p.MasterAddress != "" {
		line += ", write-as-user"
	}
	if p.GnowebURL != "" {
		line += ", gnoweb " + p.GnowebURL
	}
	return line + "\n"
}

func moreProfilesLine(n int) string {
	return fmt.Sprintf("- %d more: gno_profile_list lists every profile.\n", n)
}

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
	chainResolver := buildChainResolver(s)
	indexerResolver := buildIndexerResolver(s)

	// ---- session manager
	passphrase := os.Getenv("GNOMCP_SESSION_PASSPHRASE")
	sessionMgr := session.NewManager(*sessionsPath, passphrase)
	if err := sessionMgr.Hydrate(ctx, chainResolver); err != nil {
		log.Printf("session hydration warning: %v", err)
	}

	// ---- keystore (agent identity for local and testnet profiles)
	ks := keystore.New(defaultAgentKeysPath(), passphrase, defaultAgentMaxKeys())

	// ---- build MCP SDK server
	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "gnomcp",
		Version: version,
	}, &mcpsdk.ServerOptions{
		Instructions: buildServerInstructions(cfg.Profiles),
	})

	// ---- register + publish tools
	deps := &toolDeps{
		srv:             s,
		chainResolver:   chainResolver,
		indexerResolver: indexerResolver,
		sessionMgr:      sessionMgr,
		keystore:        ks,
		auditLog:        auditLog,
		connectClient:   &http.Client{Timeout: 10 * time.Second},
		faucetClient:    &http.Client{Timeout: 30 * time.Second},
		verifyChainID:   chain.QueryChainID,
	}
	// republishMu lives here, NOT inside the tool closure: re-registration
	// replaces gno_profile_add itself, so a closure-local mutex would be
	// swapped out on every add and serialize nothing.
	var republishMu sync.Mutex
	deps.onProfileAdded = func() error {
		republishMu.Lock()
		defer republishMu.Unlock()
		registerAllTools(deps)
		return publishTools(mcpServer, s, auditLog, *auditReads)
	}
	registerAllTools(deps)
	if err := publishTools(mcpServer, s, auditLog, *auditReads); err != nil {
		log.Fatalf("publish tools: %v", err)
	}

	// ---- run
	if err := mcpServer.Run(ctx, &mcpsdk.StdioTransport{}); err != nil {
		log.Printf("server exited: %v", err)
	}
}

// ---- resolver builders

func buildChainResolver(s *server.Server) chain.Resolver {
	type entry struct {
		rpcURL  string
		chainID string
		client  *chain.Real
	}
	var mu sync.Mutex
	cache := make(map[string]entry, len(s.Config().Profiles))
	// Eager-seed the init profiles: NewReal does no network I/O (URL parsing
	// only), so a malformed rpc-url in the loaded config still fails fast at
	// startup. Dynamic profiles are dialed lazily on first use.
	for name, p := range s.Config().Profiles {
		c, err := chain.NewReal(p.RPCURL, p.ChainID)
		if err != nil {
			log.Fatalf("chain client for profile %q: %v", name, err)
		}
		cache[name] = entry{rpcURL: p.RPCURL, chainID: p.ChainID, client: c}
	}
	return func(profile string) chain.Client {
		// Return an untyped-nil interface (not a typed-nil *chain.Real) for an
		// unresolved profile, so callers' `if c == nil` guards actually fire.
		p, ok := s.Config().Profiles[profile]
		if !ok {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		if e, ok := cache[profile]; ok && e.rpcURL == p.RPCURL && e.chainID == p.ChainID {
			return e.client
		}
		c, err := chain.NewReal(p.RPCURL, p.ChainID)
		if err != nil {
			log.Printf("chain client for profile %q: %v", profile, err)
			return nil
		}
		cache[profile] = entry{rpcURL: p.RPCURL, chainID: p.ChainID, client: c}
		return c
	}
}

func buildIndexerResolver(s *server.Server) indexer.Resolver {
	type entry struct {
		url    string
		client *indexer.GraphQL
	}
	var mu sync.Mutex
	cache := map[string]entry{}
	return func(profile string) indexer.Client {
		p, ok := s.Config().Profiles[profile]
		if !ok || p.TxIndexerURL == "" {
			return nil
		}
		mu.Lock()
		defer mu.Unlock()
		if e, ok := cache[profile]; ok && e.url == p.TxIndexerURL {
			return e.client
		}
		c := indexer.NewGraphQL(p.TxIndexerURL)
		cache[profile] = entry{url: p.TxIndexerURL, client: c}
		return c
	}
}

// ---- MCP handler adapter

func makeHandler(t *server.Tool, s *server.Server, auditLog *audit.Log, auditReads bool) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (result *mcpsdk.CallToolResult, err error) {
		// The MCP SDK invokes this handler on a bare goroutine with no recover of
		// its own, so an unrecovered panic anywhere in this adapter (argument
		// decoding, profile defaulting, result formatting, or the tool handler
		// below) would kill the whole server process. Convert any panic into one
		// tool-error result; the stack goes to stderr so the bug stays visible.
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("tool %q panicked: %v\n%s", t.Name, rec, debug.Stack())
				result = &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: fmt.Sprintf("tool %q panicked: %v", t.Name, rec)}},
					IsError: true,
				}
				err = nil
			}
		}()

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
		// JSON-Schema defaults are advisory; the client does not inject them.
		// Apply the documented profile default server-side so an omitted profile
		// resolves to a real client instead of "" (which resolves to nothing).
		applyProfileDefault(t, s, args)

		start := time.Now()
		res, callErr := s.Registry().Call(ctx, t.Name, args)
		dur := time.Since(start)

		if shouldAuditAtAdapter(t.Capability, t.SelfAudited, auditReads) {
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
			auditLog.Record(entry) // non-fatal, but logs on failure rather than silently dropping
		}

		if callErr != nil {
			return toolErrorResult(callErr), nil
		}

		return formatResult(res, t.OutputKind), nil
	}
}

// applyProfileDefault fills in the server's default profile when tool t accepts a
// `profile` argument and the caller omitted it (or passed empty). No-op when there
// is no default (e.g. zero profiles loaded, where profile is required).
func applyProfileDefault(t *server.Tool, s *server.Server, args map[string]any) {
	def := s.ProfileSchema().Default
	if def == "" || !toolAcceptsProfile(t) {
		return
	}
	if v, present := args["profile"]; present {
		str, isStr := v.(string)
		if !isStr || str != "" {
			// Explicit profile, or a malformed (non-string) value the handler
			// should reject — leave it alone.
			return
		}
	}
	args["profile"] = def
}

// toolAcceptsProfile reports whether t's input schema declares a `profile` property.
func toolAcceptsProfile(t *server.Tool) bool {
	props, ok := t.InputSchema["properties"].(map[string]any)
	if !ok {
		return false
	}
	_, ok = props["profile"]
	return ok
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
		return filepath.Join(home, ".local", "share", "gnomcp", "audit.jsonl")
	}
	return "audit.jsonl"
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

// defaultAgentMaxKeys is the per-profile cap on agent keys: GNOMCP_AGENT_MAX_KEYS
// when set to a positive integer, else 5. The cap lets a profile hold several
// keys (to exercise multi-address realms) while keeping the count bounded.
func defaultAgentMaxKeys() int {
	const fallback = 5
	if v := os.Getenv("GNOMCP_AGENT_MAX_KEYS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
		log.Printf("gnomcp: ignoring invalid GNOMCP_AGENT_MAX_KEYS=%q (want a positive integer), using %d", v, fallback)
	}
	return fallback
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
			Content:           []mcpsdk.Content{&mcpsdk.TextContent{Text: untrusted.Neutralize(te.Message)}},
			StructuredContent: sc,
			IsError:           true,
		}
	}
	// Error text is mixed-trust: gnomcp's own framing around chain/network
	// bytes (e.g. a realm panic string in an ABCI log). It is neutralized — not
	// enveloped — so embedded text cannot forge or close an envelope.
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: untrusted.Neutralize(err.Error())}},
		IsError: true,
	}
}

func argString(args map[string]any, key string) string {
	v, _ := args[key].(string)
	return v
}

// shouldAuditAtAdapter reports whether the generic MCP adapter writes an audit
// line for a tool. Self-audited tools (the write-tx tools) write their own
// enriched entry, so the adapter skips them to avoid an unlinked duplicate.
func shouldAuditAtAdapter(capability server.Capability, selfAudited, auditReads bool) bool {
	if selfAudited {
		return false
	}
	return capability == server.CapWrite || capability == server.CapWritePrep || auditReads
}

// safeArgKeys are the args allowed to appear verbatim in an audit summary.
// Every other key is redacted, so a tool added later cannot leak a sensitive
// value (function args, code, file bodies, expressions, secrets) by default —
// over-redaction is harmless for an audit log, under-redaction is not.
var safeArgKeys = map[string]bool{
	"profile": true, "realm": true, "func": true, "path": true, "file": true,
	"deploy_path": true, "simulate": true, "identity": true, "limit": true,
	"since": true, "until": true, "namespace": true, "tag": true, "category": true,
	"allow_run": true, "expires_in": true, "gnoweb_url": true, "name": true, "address": true,
	// gno_profile_add connection params: the audit line must show what chain
	// was added, and none of these carry secrets.
	"rpc_url": true, "chain_id": true, "tx_indexer_url": true,
	"faucet_service_url": true, "faucet_url": true,
}

func argsSummary(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	redacted := make(map[string]any, len(args))
	for k, v := range args {
		if safeArgKeys[k] {
			redacted[k] = v
		} else {
			redacted[k] = "[redacted]"
		}
	}
	b, err := json.Marshal(redacted)
	if err != nil {
		return ""
	}
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
