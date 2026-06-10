package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// maxRespBytes bounds the response read from the (profile-configured, possibly
// plain-http) indexer so a malicious or broken endpoint can't stream unbounded
// data into memory. The output budget truncates what reaches the LLM anyway.
const maxRespBytes = 4 << 20 // 4 MiB

// GraphQL is a Client backed by a tx-indexer GraphQL endpoint.
type GraphQL struct {
	url string
	hc  *http.Client
}

// NewGraphQL returns a GraphQL client targeting the given endpoint URL.
func NewGraphQL(url string) *GraphQL {
	return &GraphQL{
		url: url,
		hc:  &http.Client{Timeout: 10 * time.Second},
	}
}

// ---- wire types

type gqlRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

type gqlError struct {
	Message string `json:"message"`
}

type gqlEnvelope struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors,omitempty"`
}

// ---- GraphQL response types (matching the tx-indexer schema)

type gqlTransaction struct {
	Hash        string         `json:"hash"`
	BlockHeight int64          `json:"block_height"`
	Messages    []gqlTxMessage `json:"messages"`
}

type gqlTxMessage struct {
	TypeURL string          `json:"typeUrl"`
	Route   string          `json:"route"`
	Value   gqlMessageValue `json:"value"`
}

// gqlMessageValue is the union MessageValue: BankMsgSend | MsgCall | MsgAddPackage | MsgRun | UnexpectedMessage.
// We use __typename to discriminate.
type gqlMessageValue struct {
	Typename string `json:"__typename"`

	// MsgCall fields
	Caller  string   `json:"caller"`
	PkgPath string   `json:"pkg_path"`
	Func    string   `json:"func"`
	Args    []string `json:"args"`

	// MsgAddPackage fields
	Creator string         `json:"creator"`
	Package *gqlMemPackage `json:"package"`

	// MsgRun fields (caller, package shared with above)
}

type gqlMemPackage struct {
	Path string `json:"path"`
}

// ---- HTTP transport

func (c *GraphQL) do(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(gqlRequest{Query: query, Variables: vars})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("error_unavailable: indexer unreachable: %w (retry later)", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("indexer returned %d", resp.StatusCode)
	}
	var env gqlEnvelope
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxRespBytes)).Decode(&env); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if len(env.Errors) > 0 {
		return fmt.Errorf("graphql error: %s", env.Errors[0].Message)
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return fmt.Errorf("unmarshal data: %w", err)
	}
	return nil
}

// ---- query strings

// transactionsQuery queries transactions filtered by pkg_path using the deprecated
// `transactions` field. The tx-indexer schema has no `getTransactions` in the public
// schema file; the deprecated field remains the only stable query entry point.
//
// We request __typename on the value union so we can discriminate the type in Go.
const transactionsQuery = `
query Transactions($filter: TransactionFilter!) {
  transactions(filter: $filter) {
    hash
    block_height
    messages {
      typeUrl
      route
      value {
        __typename
        ... on MsgCall {
          caller
          pkg_path
          func
          args
        }
        ... on MsgAddPackage {
          creator
          package { path }
        }
        ... on MsgRun {
          caller
          package { path }
        }
      }
    }
  }
}`

// ---- interface methods

// List returns realms matching the filter. The tx-indexer schema does not expose a
// `realms` query as of this writing; this method returns an error until the metadata
// indexing extension lands in the schema.
func (c *GraphQL) List(_ context.Context, _ ListFilter) ([]Realm, error) {
	return nil, fmt.Errorf("indexer: List not supported by this indexer (realms query not yet in schema)")
}

// History returns every transaction touching realm in chronological order.
func (c *GraphQL) History(ctx context.Context, realm string) ([]TxEvent, error) {
	vars := map[string]any{
		"filter": map[string]any{
			"message": []any{
				map[string]any{
					"route": "vm",
					"vm_param": map[string]any{
						"exec": map[string]any{"pkg_path": realm},
					},
				},
				map[string]any{
					"route": "vm",
					"vm_param": map[string]any{
						"add_package": map[string]any{
							"package": map[string]any{"path": realm},
						},
					},
				},
				map[string]any{
					"route": "vm",
					"vm_param": map[string]any{
						"run": map[string]any{
							"package": map[string]any{"path": realm},
						},
					},
				},
			},
		},
	}

	var data struct {
		Transactions []gqlTransaction `json:"transactions"`
	}
	if err := c.do(ctx, transactionsQuery, vars, &data); err != nil {
		return nil, err
	}
	return toTxEvents(data.Transactions), nil
}

// Activity returns MsgCall and MsgRun events for realm.
//
// Time filtering (since/until) is not supported by the current tx-indexer schema:
// the Transaction type at gnolang/tx-indexer serve/graph/schema/types/transaction.graphql
// exposes only block_height — there is no Block relation on Transaction, and Block.time
// is reachable only via a separate `blocks` query. Passing a non-nil since or until
// therefore returns error_unavailable rather than silently filtering against a zero
// TxEvent.Time. A future enhancement could batch-fetch Block.time via a follow-up
// query and apply the time predicate; that work is deferred.
//
// When both since and until are nil, this method returns History filtered to
// MsgCall/MsgRun only (MsgAddPackage is excluded per the Activity contract).
func (c *GraphQL) Activity(ctx context.Context, realm string, since, until *time.Time) ([]TxEvent, error) {
	if since != nil || until != nil {
		return nil, fmt.Errorf("error_unavailable: time filtering not supported by current indexer schema (no time field on Transaction)")
	}
	all, err := c.History(ctx, realm)
	if err != nil {
		return nil, err
	}
	out := make([]TxEvent, 0, len(all))
	for _, e := range all {
		if e.Kind == "MsgCall" || e.Kind == "MsgRun" {
			out = append(out, e)
		}
	}
	return out, nil
}

// ---- conversion helpers

func toTxEvents(txs []gqlTransaction) []TxEvent {
	events := make([]TxEvent, 0, len(txs))
	for _, tx := range txs {
		ev := TxEvent{
			Hash:   tx.Hash,
			Height: tx.BlockHeight,
		}
		// Use the first message to populate kind, caller, func, args.
		if len(tx.Messages) > 0 {
			populateFromMessage(&ev, tx.Messages[0])
		}
		events = append(events, ev)
	}
	return events
}

func populateFromMessage(ev *TxEvent, msg gqlTxMessage) {
	v := msg.Value
	switch v.Typename {
	case "MsgCall":
		ev.Kind = "MsgCall"
		ev.Caller = v.Caller
		ev.Func = v.Func
		ev.Args = v.Args
	case "MsgAddPackage":
		ev.Kind = "MsgAddPackage"
		ev.Caller = v.Creator
	case "MsgRun":
		ev.Kind = "MsgRun"
		ev.Caller = v.Caller
	case "BankMsgSend":
		ev.Kind = "BankMsgSend"
	default:
		// Fallback: use the typeUrl from the message envelope.
		ev.Kind = msg.TypeURL
	}
}

var _ Client = (*GraphQL)(nil)
