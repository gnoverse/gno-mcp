package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gnolang/gno/gno.land/pkg/gnoweb"
)

// GnowebConfig carries the gnoweb half of simnet. AdvertisedRPC feeds the
// gnoconnect:rpc <head> meta-tag that gno_profile_add discovers from — it
// MUST be the alias-hostname URL, never 127.0.0.1: gnomcp rejects a
// non-loopback gnoweb advertising a loopback RPC (gnowebRPCUnusable).
type GnowebConfig struct {
	NodeRPC       string // node RPC for gnoweb's own queries (tcp://... from Boot is fine)
	AdvertisedRPC string // what gnoconnect:rpc advertises (http://<alias>:<port>)
	ChainID       string
	Listen        string // e.g. 127.0.0.1:8688; :0 for tests
}

// StartGnoweb serves the gnoweb UI on c.Listen. Returns the actual listen
// address and a shutdown func.
func StartGnoweb(logger *slog.Logger, c GnowebConfig) (string, func(), error) {
	appcfg := gnoweb.NewDefaultAppConfig()
	appcfg.NodeRemote = c.NodeRPC
	appcfg.RemoteHelp = c.AdvertisedRPC // feeds gnoconnect:rpc
	appcfg.ChainID = c.ChainID          // feeds gnoconnect:chainid; set explicitly so router creation needs no live fetch

	router, err := gnoweb.NewRouter(logger, appcfg)
	if err != nil {
		return "", nil, fmt.Errorf("gnoweb router: %w", err)
	}

	ln, err := net.Listen("tcp", c.Listen)
	if err != nil {
		return "", nil, fmt.Errorf("gnoweb listen %s: %w", c.Listen, err)
	}
	srv := &http.Server{Handler: router, ReadTimeout: 10 * time.Second, WriteTimeout: 30 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("gnoweb server", "err", err)
		}
	}()
	return ln.Addr().String(), func() { _ = srv.Close() }, nil
}
