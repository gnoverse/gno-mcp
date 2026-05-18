package tools

import (
	"github.com/gnolang/gno-mcp/internal/audit"
	"github.com/gnolang/gno-mcp/internal/client"
	"github.com/gnolang/gno-mcp/internal/session"
	"github.com/mark3labs/mcp-go/server"
)

type Deps struct {
	Client  client.GnopieClient
	Audit   *audit.Log
	Session *session.Manager
}

type Registrar func(s *server.MCPServer, d Deps)

var All []Registrar

func Register(r Registrar) { All = append(All, r) }

func RegisterAll(s *server.MCPServer, d Deps) {
	for _, r := range All {
		r(s, d)
	}
}
