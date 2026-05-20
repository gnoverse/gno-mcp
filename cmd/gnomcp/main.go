// Command gnomcp is the Gno Model Context Protocol server.
// It exposes Gno chain read tools to MCP clients via stdio transport.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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
	"github.com/gnoverse/gno-mcp/internal/profiles"
	"github.com/gnoverse/gno-mcp/internal/server"
	idxtools "github.com/gnoverse/gno-mcp/internal/tools/indexer"
	readtools "github.com/gnoverse/gno-mcp/internal/tools/read"
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
		}
	}

	fs := flag.NewFlagSet("gnomcp", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath(), "path to profiles.toml")
	auditPath := fs.String("audit-path", defaultAuditPath(), "path to audit log file")
	auditReads := fs.Bool("audit-reads", false, "also audit read-only tool calls")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("flag parse: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ---- load + validate profiles
	f, err := os.Open(*configPath)
	if err != nil {
		log.Fatalf("open config: %v", err)
	}
	cfg, err := profiles.Load(f)
	f.Close()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("validate config: %v", err)
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

	// ---- register tools
	readtools.RegisterRender(s, chainResolver)
	readtools.RegisterEval(s, chainResolver)
	readtools.RegisterRead(s, chainResolver)
	readtools.RegisterInspect(s, chainResolver)

	if s.AnyProfileHasIndexer() {
		idxtools.RegisterList(s, indexerResolver)
		idxtools.RegisterHistory(s, indexerResolver)
		idxtools.RegisterActivity(s, indexerResolver)
	}

	// ---- build MCP SDK server
	mcpServer := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "gnomcp",
		Version: version,
	}, nil)

	for _, t := range s.Registry().All() {
		t := t // capture for closure
		inputSchema, err := mapToSchema(t.InputSchema)
		if err != nil {
			log.Fatalf("build schema for tool %q: %v", t.Name, err)
		}
		mcpServer.AddTool(&mcpsdk.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: inputSchema,
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
				Duration:    dur,
			}
			if callErr != nil {
				entry.Result = "tool_err"
			} else {
				entry.Result = "ok"
			}
			_ = auditLog.Append(entry) // audit errors are non-fatal
		}

		if callErr != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: callErr.Error()}},
				IsError: true,
			}, nil
		}

		return formatResult(res, t.OutputKind), nil
	}
}

// formatResult converts a server.Result to a *mcp.CallToolResult.
func formatResult(res server.Result, kind server.OutputKind) *mcpsdk.CallToolResult {
	switch kind {
	case server.OutputResource:
		mime := res.ResourceMIME
		if mime == "" {
			mime = "text/markdown"
		}
		return &mcpsdk.CallToolResult{
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
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: res.Text}},
		}
	}
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
	discoverCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	for name, p := range cfg.Profiles {
		ok, err := profiles.DiscoverLocal(discoverCtx, p, 1*time.Second)
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

func defaultConfigPath() string {
	if v := os.Getenv("GNOMCP_CONFIG"); v != "" {
		return v
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "gnomcp", "profiles.toml")
	}
	return "profiles.toml"
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

// ---- helpers

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
