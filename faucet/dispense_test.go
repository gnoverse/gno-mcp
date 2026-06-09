package faucet

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGnoclientDispenser_badRecipient(t *testing.T) {
	d := &gnoclientDispenser{} // cli nil is fine — recipient parse fails first
	_, err := d.Send(context.Background(), "not-bech32", 1_000_000)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "recipient")
}
