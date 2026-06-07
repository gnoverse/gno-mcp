package write

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/server"
	"github.com/gnoverse/gno-mcp/internal/session"
)

// RegisterAuthStatus registers the gno_auth_status tool.
// sessionMgr holds the local session state; resolver provides the chain client
// for per-session status refresh.
func RegisterAuthStatus(s *server.Server, sessionMgr *session.Manager, resolver chain.Resolver) {
	s.Registry().Add(&server.Tool{
		Name: "gno_auth_status",
		Description: "Lists all gnomcp-managed sessions for a profile, refreshing each session's " +
			"state from the chain. Pending sessions that the chain reports as active are " +
			"transitioned locally. Returns a prose narrative and a structuredContent sessions " +
			"array. Use this to check whether a session is active before attempting a write, " +
			"or to diagnose why a write failed. Required args: profile.",
		InputSchema: authStatusInputSchema(s),
		OutputKind:  server.OutputText,
		Capability:  server.CapWritePrep,
		Annotations: server.Annotations{
			ReadOnly:    true,
			Destructive: false,
			Idempotent:  true,
			OpenWorld:   false,
		},
		Handler: func(ctx context.Context, args map[string]any) (server.Result, error) {
			return authStatusHandler(ctx, args, s, sessionMgr, resolver)
		},
	})
}

func authStatusHandler(
	ctx context.Context,
	args map[string]any,
	s *server.Server,
	sessionMgr *session.Manager,
	resolver chain.Resolver,
) (server.Result, error) {
	profileName, err := stringArg(args, "profile")
	if err != nil {
		return server.Result{}, err
	}
	if profileName == "" {
		return server.Result{}, fmt.Errorf("profile: required — pick one of the configured profiles")
	}

	if _, ok := s.Config().Profiles[profileName]; !ok {
		return server.Result{}, fmt.Errorf("profile %q: not found", profileName)
	}

	metas := sessionMgr.ListForProfile(profileName)

	// Refresh each session from chain. Tolerate per-session errors.
	type sessionInfo struct {
		meta  *session.SessionMeta
		label string // "[active]", "[pending]", "[expired]", "[revoked]"
	}
	infos := make([]sessionInfo, 0, len(metas))

	client := resolver(profileName)
	for _, meta := range metas {
		info := sessionInfo{meta: meta}

		if client != nil && meta.MasterAddress != "" {
			status, queryErr := client.QuerySession(ctx, meta.MasterAddress, meta.SessionAddress)
			if queryErr == nil {
				// Chain answered — apply transitions.
				switch {
				case status.Active && meta.State == session.StatePending:
					// Promote pending → active in the manager.
					if markErr := sessionMgr.MarkActive(profileName, meta.SessionAddress, status); markErr == nil {
						meta.State = session.StateActive
					} else {
						log.Printf("gno_auth_status: mark active %q: %v (showing pending)", meta.SessionAddress, markErr)
					}
				case !status.Active && meta.State == session.StateActive:
					// Previously active, but the chain no longer reports it active
					// → revoked or expired. (A still-pending session also reads
					// !Active before authorization, so only downgrade an
					// already-active one to avoid revoking un-authorized pendings.)
					newState := session.StateRevoked
					if meta.ExpiresAt > 0 && time.Now().Unix() >= meta.ExpiresAt {
						newState = session.StateExpired
					}
					if markErr := sessionMgr.MarkInactive(profileName, meta.SessionAddress, newState); markErr == nil {
						meta.State = newState
					} else {
						log.Printf("gno_auth_status: mark inactive %q: %v", meta.SessionAddress, markErr)
					}
				}
			}
		}

		switch meta.State {
		case session.StateActive:
			info.label = "[active]"
		case session.StatePending:
			info.label = "[pending]"
		case session.StateExpired:
			info.label = "[expired]"
		case session.StateRevoked:
			info.label = "[revoked]"
		default:
			info.label = "[" + meta.State + "]"
		}
		infos = append(infos, info)
	}

	// Build prose narrative.
	var b strings.Builder
	fmt.Fprintf(&b, "Auth status for profile %q\n\n", profileName)

	if len(infos) == 0 {
		fmt.Fprintf(&b,
			"No sessions found — no active session for profile %q.\n\n"+
				"Use gno_session_propose to create one.\n",
			profileName,
		)
	} else {
		for _, info := range infos {
			fmt.Fprintf(&b, "%s %s\n", info.label, info.meta.SessionAddress)
			fmt.Fprintf(&b, "  paths:   %s\n", strings.Join(info.meta.AllowPaths, ", "))
			if info.meta.SpendLimit != "" {
				fmt.Fprintf(&b, "  limit:   %s\n", info.meta.SpendLimit)
			}
			if info.meta.SpendRemaining != "" {
				fmt.Fprintf(&b, "  remaining: %s\n", info.meta.SpendRemaining)
			}
			b.WriteString("\n")
		}
	}

	// Build structuredContent sessions array.
	sessions := make([]map[string]any, 0, len(infos))
	for _, info := range infos {
		sessions = append(sessions, map[string]any{
			"state":           info.meta.State,
			"session_address": info.meta.SessionAddress,
			"session_pubkey":  info.meta.SessionPubkey,
			"allow_paths":     info.meta.AllowPaths,
			"spend_limit":     info.meta.SpendLimit,
			"spend_remaining": info.meta.SpendRemaining,
			"expires_at":      info.meta.ExpiresAt,
		})
	}

	return server.Result{
		Text: b.String(),
		StructuredContent: map[string]any{
			"profile":  profileName,
			"sessions": sessions,
		},
	}, nil
}

func authStatusInputSchema(s *server.Server) map[string]any {
	props := map[string]any{}
	required := []string{}
	addProfileArg(s, props, &required)
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}
