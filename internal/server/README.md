# MCP Go SDK API Notes

SDK: `github.com/modelcontextprotocol/go-sdk` v1.4.1 (see go.mod for the pinned version).
Import: `"github.com/modelcontextprotocol/go-sdk/mcp"`

---

## Server construction

```go
server := mcp.NewServer(&mcp.Implementation{Name: "gnomcp", Version: "v0.2.0"}, nil)
```

`NewServer(impl *mcp.Implementation, options *mcp.ServerOptions) *mcp.Server`

`mcp.Implementation` fields: `Name`, `Title`, `Version`, `WebsiteURL`, `Icons`.
`options` is `*mcp.ServerOptions` (nil for defaults). Relevant fields:
- `Instructions string` — optional instructions sent to clients
- `Logger *slog.Logger` — server-level logger
- `KeepAlive time.Duration` — ping interval for dead-connection detection

---

## Tool registration

Two paths:

**Generic (preferred): `mcp.AddTool[In, Out]`**

```go
type Args struct {
    Path string `json:"path" jsonschema:"the realm path to render"`
}

mcp.AddTool(server, &mcp.Tool{
    Name:        "gno_render",
    Description: "Render a Gno realm at a given path",
}, func(ctx context.Context, req *mcp.CallToolRequest, args Args) (*mcp.CallToolResult, any, error) {
    // args is already unmarshaled and validated against the inferred JSON schema
    return &mcp.CallToolResult{
        Content: []mcp.Content{&mcp.TextContent{Text: "..."}},
    }, nil, nil
})
```

`ToolHandlerFor[In, Out any]` signature:
```
func(ctx context.Context, req *mcp.CallToolRequest, input In) (result *mcp.CallToolResult, output Out, _ error)
```

- `In` must be a struct or map (object schema). Use `any` to skip schema inference.
- `Out` type is used for structured output schema. Use `any` to omit.
- Returning a nil `*mcp.CallToolResult` is valid if only `Out` or `error` matter.
- A returned `error` is packed as a tool error (`IsError: true`) in Content — NOT a protocol error.

**Untyped fallback: `server.AddTool`**

```go
server.AddTool(&mcp.Tool{Name: "..."}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    // manual arg parsing from req.Params.Arguments
    ...
})
```

`ToolHandler` signature: `func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)`

---

## Content types

`mcp.Content` is an interface. Concrete types:

| Type | Fields |
|---|---|
| `*mcp.TextContent` | `Text string`, `Meta`, `Annotations *Annotations` |
| `*mcp.ImageContent` | `Data []byte` (raw, marshaled as base64), `MIMEType string`, `Meta`, `Annotations` |
| `*mcp.EmbeddedResource` | `Resource *mcp.ResourceContents`, `Meta`, `Annotations` |
| `*mcp.ResourceLink` | (links to a resource URI without embedding data) |

`CallToolResult.Content` is `[]mcp.Content`.

---

## CallToolResult shape

```go
type CallToolResult struct {
    Content           []mcp.Content `json:"content"`
    StructuredContent any           `json:"structuredContent,omitempty"`
    IsError           bool          `json:"isError,omitempty"`
}
```

Helper methods: `r.SetError(err)`, `r.GetError() error`.

---

## Stdio transport (run mode)

```go
if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
    log.Printf("server failed: %v", err)
}
```

`StdioTransport` is a zero-value struct (`type StdioTransport struct{}`).
`server.Run(ctx, transport)` blocks until the client closes the connection or `ctx` is cancelled.

For HTTP/SSE (future), use `mcp.NewSSEHandler` or `mcp.NewStreamableHTTPHandler`.

---

## Notes

- `mcp.CallToolRequest` is a type alias: `type CallToolRequest = ServerRequest[*CallToolParamsRaw]`.
- `Server.AddTool` takes the untyped `ToolHandler`; `mcp.AddTool` (package-level func) takes the generic `ToolHandlerFor[In,Out]`.
- Both approaches coexist; use the generic form whenever input types are known.
- JSON schema inference uses `github.com/google/jsonschema-go` (2020-12 draft only).
