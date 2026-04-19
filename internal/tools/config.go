package tools

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/gnolang/gno-mcp/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func init() {
	Register(registerConfigGet)
	Register(registerConfigSet)
}

var allowedConfigKeys = map[string]bool{
	"default_key":     true,
	"default_network": true,
	"gas_buffer":      true,
}

func registerConfigGet(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_config_get",
		mcp.WithDescription("Get the current gno-mcp configuration as JSON."),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		b, _ := json.Marshal(cfg)
		return mcp.NewToolResultText(string(b)), nil
	})
}

func registerConfigSet(s *server.MCPServer, d Deps) {
	t := mcp.NewTool(
		"gno_config_set",
		mcp.WithDescription("Set a gno-mcp configuration value. Allowed keys: default_key, default_network, gas_buffer."),
		mcp.WithString("key", mcp.Required(), mcp.Description("Config key: default_key, default_network, or gas_buffer")),
		mcp.WithString("value", mcp.Required(), mcp.Description("New value for the config key")),
	)
	s.AddTool(t, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		key := req.GetString("key", "")
		value := req.GetString("value", "")

		if !allowedConfigKeys[key] {
			return mcp.NewToolResultError("unknown config key: " + key), nil
		}

		cfg, err := config.Load()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		switch key {
		case "default_key":
			cfg.DefaultKey = value
		case "default_network":
			cfg.DefaultNetwork = value
		case "gas_buffer":
			n, _ := strconv.Atoi(value)
			cfg.GasBuffer = n
		}

		if err := config.Save(cfg); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(`{"status":"ok","key":"` + key + `","value":"` + value + `"}`), nil
	})
}
