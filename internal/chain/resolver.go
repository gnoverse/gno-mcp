package chain

// Resolver returns the Client to use for a given profile name.
// The caller wires this up — typically maps profile name to a Real
// instance constructed from the profile's RPC URL. Tools use
// chain.Resolver as the dependency-injection point for the chain
// client so they remain agnostic to how clients are constructed and
// cached.
type Resolver func(profile string) Client
