package clientfaucet

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

func TestResolve_precedence(t *testing.T) {
	fake := chain.NewFake()
	hc := &http.Client{}
	svc := Resolve(profiles.Profile{FaucetServiceURL: "http://x", FaucetURL: "http://y"}, fake, hc)
	assert.IsType(t, &ServiceFaucet{}, svc, "service-url wins")
	link := Resolve(profiles.Profile{FaucetURL: "http://y"}, fake, hc)
	assert.IsType(t, &LinkFaucet{}, link, "faucet-url -> link")
	fallback := Resolve(profiles.Profile{}, fake, hc)
	assert.IsType(t, &LinkFaucet{}, fallback, "neither -> link (empty url = manual fallback)")
}
