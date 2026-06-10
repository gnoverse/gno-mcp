package session

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gnoverse/gno-mcp/internal/chain"
	"github.com/gnoverse/gno-mcp/internal/profiles"
)

// Scope is the resolved, clamped set of constraints for a session.
type Scope struct {
	SpendLimit  string
	SpendPeriod time.Duration
	ExpiresIn   time.Duration
	AllowPaths  []string
	AllowRun    bool
}

// ScopeArgs carries the agent-supplied values from gno_session_propose.
// Empty string means "agent omitted"; ResolveScope falls back to profile
// defaults and then hardcoded fallback. At least one of AllowPaths
// (non-empty) or AllowRun==true must be requested.
type ScopeArgs struct {
	SpendLimit string
	ExpiresIn  string
	AllowPaths []string
	AllowRun   bool
}

const (
	defaultSpendLimit = "100000ugnot"
	defaultExpiresIn  = time.Hour
)

// ResolveScope applies the 4-layer scope policy.
func ResolveScope(args ScopeArgs, profile *profiles.Profile) (Scope, []string, error) {
	if len(args.AllowPaths) == 0 && !args.AllowRun {
		return Scope{}, nil, errors.New(
			"session: at least one of allow_paths (non-empty, e.g. " +
				"[\"gno.land/r/myorg/blog\"]) or allow_run=true must be requested",
		)
	}

	for _, p := range args.AllowPaths {
		if err := validateAllowPath(p); err != nil {
			return Scope{}, nil, err
		}
	}

	profileSpendLimit, profileExpiresIn, err := profile.EffectiveDefaults()
	if err != nil {
		return Scope{}, nil, fmt.Errorf("session: profile effective defaults: %w", err)
	}

	spendLimit := profileSpendLimit
	if args.SpendLimit != "" {
		spendLimit = args.SpendLimit
	}
	// Validate unconditionally: clampCoin runs only when a hard cap is set and is
	// skipped entirely on the BypassHardLimits path, so without this check a
	// shell-metacharacter spend_limit reaches the pasted gnokey command.
	if !profiles.SpendLimitValid(spendLimit) {
		return Scope{}, nil, fmt.Errorf("session: spend_limit %q is invalid (want digits then a denom, e.g. 1000000ugnot)", spendLimit)
	}

	expiresIn := profileExpiresIn
	if args.ExpiresIn != "" {
		d, err := time.ParseDuration(args.ExpiresIn)
		if err != nil {
			return Scope{}, nil, fmt.Errorf("session: expires_in %q: %w", args.ExpiresIn, err)
		}
		expiresIn = d
	}

	scope := Scope{
		SpendLimit:  spendLimit,
		SpendPeriod: expiresIn,
		ExpiresIn:   expiresIn,
		AllowPaths:  args.AllowPaths,
		AllowRun:    args.AllowRun,
	}

	if profile.BypassHardLimits {
		return scope, nil, nil
	}

	limits := profile.HardLimits()
	var warnings []string

	if limits.MaxSpendLimit != "" {
		clamped, didClamp, err := clampCoin(scope.SpendLimit, limits.MaxSpendLimit)
		if err != nil {
			return Scope{}, nil, fmt.Errorf("session: clamp spend_limit: %w", err)
		}
		if didClamp {
			warnings = append(warnings, fmt.Sprintf(
				"WARNING: requested spend_limit %s exceeds %s cap of %s; clamped to %s.",
				scope.SpendLimit, profile.ChainType, limits.MaxSpendLimit, clamped,
			))
			scope.SpendLimit = clamped
		}
	}

	if limits.MaxExpiresIn > 0 && scope.ExpiresIn > limits.MaxExpiresIn {
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: requested expires_in %s exceeds %s cap of %s; clamped to %s.",
			scope.ExpiresIn, profile.ChainType, limits.MaxExpiresIn, limits.MaxExpiresIn,
		))
		scope.ExpiresIn = limits.MaxExpiresIn
		scope.SpendPeriod = limits.MaxExpiresIn
	}

	if limits.MaxAllowPathsCount > 0 && len(scope.AllowPaths) > limits.MaxAllowPathsCount {
		warnings = append(warnings, fmt.Sprintf(
			"WARNING: requested %d allow_paths exceeds %s cap of %d; clamped to first %d.",
			len(scope.AllowPaths), profile.ChainType, limits.MaxAllowPathsCount, limits.MaxAllowPathsCount,
		))
		scope.AllowPaths = scope.AllowPaths[:limits.MaxAllowPathsCount]
	}

	return scope, warnings, nil
}

// allowPathUnsafe matches any character that must never appear in an allow_paths
// entry. Realm paths use only lowercase letters, digits, '.', '/', '_', and '-';
// anything else (whitespace, shell metacharacters) signals an injection attempt
// into the gnokey command the user pastes into a terminal.
var allowPathUnsafe = regexp.MustCompile(`[^a-z0-9./_-]`)

// validateAllowPath rejects an allow_paths entry that is not a clean realm path.
// The character allowlist closes shell-injection into the pasted gnokey command;
// chain.IsRealmPath enforces that the entry is a callable realm (the vm/exec
// target the entry is rendered as).
func validateAllowPath(p string) error {
	if p == "" {
		return errors.New("session: allow_paths entry is empty")
	}
	if allowPathUnsafe.MatchString(p) {
		return fmt.Errorf("session: allow_paths entry %q contains illegal characters (want a realm path like gno.land/r/org/name)", p)
	}
	if !chain.IsRealmPath(p) {
		return fmt.Errorf("session: allow_paths entry %q is not a realm path (want gno.land/r/...)", p)
	}
	return nil
}

func clampCoin(a, cap string) (string, bool, error) {
	aMag, aDenom, err := parseCoins(a)
	if err != nil {
		return "", false, fmt.Errorf("parse %q: %w", a, err)
	}
	cMag, cDenom, err := parseCoins(cap)
	if err != nil {
		return "", false, fmt.Errorf("parse cap %q: %w", cap, err)
	}
	if aDenom != cDenom {
		return "", false, fmt.Errorf(
			"can't compare across denominations: %q vs %q", aDenom, cDenom,
		)
	}
	if aMag > cMag {
		return cap, true, nil
	}
	return a, false, nil
}

func parseCoins(s string) (mag int64, denom string, err error) {
	if s == "" {
		return 0, "", errors.New("empty coin string")
	}
	split := strings.IndexFunc(s, func(r rune) bool {
		return r < '0' || r > '9'
	})
	if split <= 0 {
		return 0, "", fmt.Errorf("coin %q: missing denomination", s)
	}
	mag, err = strconv.ParseInt(s[:split], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("coin %q: parse magnitude: %w", s, err)
	}
	denom = s[split:]
	if denom == "" {
		return 0, "", fmt.Errorf("coin %q: empty denomination", s)
	}
	return mag, denom, nil
}
