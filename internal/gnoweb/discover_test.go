package gnoweb

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscover_ParsesMetaTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head>
<meta name="gnoconnect:rpc" content="https://rpc.test13.testnets.gno.land" />
<meta name="gnoconnect:chainid" content="test-13" />
</head></html>`))
	}))
	defer srv.Close()

	got, err := Discover(srv.Client(), srv.URL)
	require.NoError(t, err, "discover")
	assert.Equal(t, "https://rpc.test13.testnets.gno.land", got.RPC)
	assert.Equal(t, "test-13", got.ChainID)
}

func TestDiscover_MissingTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head></head></html>`))
	}))
	defer srv.Close()
	_, err := Discover(srv.Client(), srv.URL)
	require.Error(t, err, "expected error when gnoconnect meta-tags are absent")
}

func TestDiscover_HeadOnly_IgnoresBodyMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decoy gnoconnect tags in <body> must be ignored: discovery parses the
		// <head> only, so the real head values win.
		w.Write([]byte(`<html><head>
<meta name="gnoconnect:rpc" content="https://rpc.test13.testnets.gno.land" />
<meta name="gnoconnect:chainid" content="test-13" />
</head><body>
<meta name="gnoconnect:rpc" content="https://decoy.example.com" />
<meta name="gnoconnect:chainid" content="decoy" />
</body></html>`))
	}))
	defer srv.Close()
	got, err := Discover(srv.Client(), srv.URL)
	require.NoError(t, err, "discover")
	assert.Equal(t, "https://rpc.test13.testnets.gno.land", got.RPC, "body decoy should be ignored (head-only parse)")
	assert.Equal(t, "test-13", got.ChainID, "body decoy should be ignored (head-only parse)")
}

func TestDiscover_AttributeOrderIndependent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// content before name, and a single-quoted value — must still parse.
		w.Write([]byte(`<meta content='https://rpc.test13.testnets.gno.land' name="gnoconnect:rpc">` +
			`<meta content="test-13" name="gnoconnect:chainid">`))
	}))
	defer srv.Close()
	got, err := Discover(srv.Client(), srv.URL)
	require.NoError(t, err, "discover")
	assert.NotEmpty(t, got.RPC, "reversed-order parse failed: %+v", got)
	assert.Equal(t, "test-13", got.ChainID, "reversed-order parse failed: %+v", got)
}

func TestDiscover_404WithMetaTags(t *testing.T) {
	// gnoweb embeds gnoconnect meta-tags on every page it serves, including its
	// 404 error page. A fresh chain's root URL returns 404 (gnoweb redirects /
	// to /r/gnoland/home, which is undeployed on fresh chains). Discover must
	// parse the body regardless of HTTP status and succeed when both tags are found.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<html><head>
<meta name="gnoconnect:rpc" content="https://rpc.test13.testnets.gno.land" />
<meta name="gnoconnect:chainid" content="test-13" />
</head><body><h1>404 Not Found</h1></body></html>`))
	}))
	defer srv.Close()

	got, err := Discover(srv.Client(), srv.URL)
	require.NoError(t, err, "discover must succeed when gnoconnect tags are present even on a 404 page")
	assert.Equal(t, "https://rpc.test13.testnets.gno.land", got.RPC)
	assert.Equal(t, "test-13", got.ChainID)
}

func TestDiscover_404WithoutMetaTags(t *testing.T) {
	// A non-2xx response with no gnoconnect tags should still error, and the
	// error message should include the HTTP status so the caller can diagnose.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`<html><head><title>not gnoweb</title></head></html>`))
	}))
	defer srv.Close()

	_, err := Discover(srv.Client(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404", "error for a tagless non-2xx response must include the HTTP status")
}
