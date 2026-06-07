package gnoweb

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscover_ParsesMetaTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head>
<meta name="gnoconnect:rpc" content="https://rpc.test11.testnets.gno.land" />
<meta name="gnoconnect:chainid" content="test11" />
</head></html>`))
	}))
	defer srv.Close()

	got, err := Discover(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got.RPC != "https://rpc.test11.testnets.gno.land" {
		t.Errorf("rpc = %q", got.RPC)
	}
	if got.ChainID != "test11" {
		t.Errorf("chainid = %q", got.ChainID)
	}
}

func TestDiscover_MissingTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head></head></html>`))
	}))
	defer srv.Close()
	if _, err := Discover(srv.Client(), srv.URL); err == nil {
		t.Fatal("expected error when gnoconnect meta-tags are absent")
	}
}

func TestDiscover_AttributeOrderIndependent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// content before name, and a single-quoted value — must still parse.
		w.Write([]byte(`<meta content='https://rpc.test11.testnets.gno.land' name="gnoconnect:rpc">` +
			`<meta content="test11" name="gnoconnect:chainid">`))
	}))
	defer srv.Close()
	got, err := Discover(srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if got.RPC == "" || got.ChainID != "test11" {
		t.Errorf("reversed-order parse failed: %+v", got)
	}
}
