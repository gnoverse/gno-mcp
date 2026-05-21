package session

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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

	profileSpendLimit, profileExpiresIn, err := profile.EffectiveDefaults()
	if err != nil {
		return Scope{}, nil, fmt.Errorf("session: profile effective defaults: %w", err)
	}

	spendLimit := profileSpendLimit
	if args.SpendLimit != "" {
		spendLimit = args.SpendLimit
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
