package indexer

// Resolver returns the Client to use for a given profile name.
// nil = profile has no tx-indexer-url configured; the caller should
// surface a clear error. Tools take Resolver as the DI seam for the
// indexer client.
type Resolver func(profile string) Client
