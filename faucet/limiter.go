package faucet

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrCooldown    = errors.New("faucet: address in cooldown")
	ErrRateLimited = errors.New("faucet: per-IP rate limit")
	ErrDailyCap    = errors.New("faucet: global daily cap reached")
	ErrDripLimited = errors.New("faucet: global drip rate exceeded")
)

// LimiterCfg configures the rate limiter.
// Defaults: Now→time.Now, PerAddrWindow→24h, PerIPWindow→1h when zero.
// PerAddrMax, PerIPMax, and DailyCapUgnot are REQUIRED — a zero value blocks
// all grants (fail-closed). Callers must set them explicitly.
type LimiterCfg struct {
	Now           func() time.Time // nil -> time.Now
	PerAddrWindow time.Duration    // 0 -> 24h
	PerAddrMax    int              // max grants per address per window
	PerIPWindow   time.Duration    // 0 -> 1h
	PerIPMax      int              // max grants per IP per window
	DailyCapUgnot int64            // hard ceiling on total daily outflow
	GrantUgnot    int64            // amount counted per successful Allow

	// Global drip: a token bucket over total ugnot outflow, independent of
	// address or IP. DripBurstUgnot is the bucket capacity (the largest spike
	// tolerated); DripRateUgnotPerSec is the sustained refill. A zero
	// DripBurstUgnot disables the drip control (fail-open for this one check —
	// the daily cap still bounds total outflow).
	DripBurstUgnot      int64
	DripRateUgnotPerSec int64
}

// Limiter is a clock-injectable, mutex-guarded in-memory rate limiter that
// tracks per-address and per-IP sliding windows plus a hard global daily cap.
type Limiter struct {
	now           func() time.Time
	perAddrWindow time.Duration
	perAddrMax    int
	perIPWindow   time.Duration
	perIPMax      int
	dailyCapUgnot int64
	grantUgnot    int64

	addrHits map[string][]time.Time
	ipHits   map[string][]time.Time
	daySpent int64
	dayStart time.Time

	// Global drip token bucket (ugnot). dripTokens is refilled at dripRatePerSec
	// up to dripBurst and spent by grantUgnot on each Allow. In-memory, like the
	// daily cap and the cooldown windows: these limits bound outflow only over a
	// SINGLE process lifetime. A restart re-seeds the bucket to full burst, zeroes
	// daySpent, and forgives every cooldown — so a crash-loop multiplies the
	// effective budget. For a long-lived single process this is fine; a
	// multi-replica or restart-resilient faucet must move this state to a shared
	// durable store keyed on a single global key (GCRA-on-Redis or a DB counter).
	dripTokens     float64
	dripBurst      float64
	dripRatePerSec float64
	dripLast       time.Time

	mu sync.Mutex
}

// NewLimiter constructs a Limiter with the given config, applying defaults for
// zero-valued duration and clock fields.
func NewLimiter(cfg LimiterCfg) *Limiter {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.PerAddrWindow == 0 {
		cfg.PerAddrWindow = 24 * time.Hour
	}
	if cfg.PerIPWindow == 0 {
		cfg.PerIPWindow = time.Hour
	}
	return &Limiter{
		now:            cfg.Now,
		perAddrWindow:  cfg.PerAddrWindow,
		perAddrMax:     cfg.PerAddrMax,
		perIPWindow:    cfg.PerIPWindow,
		perIPMax:       cfg.PerIPMax,
		dailyCapUgnot:  cfg.DailyCapUgnot,
		grantUgnot:     cfg.GrantUgnot,
		dripBurst:      float64(cfg.DripBurstUgnot),
		dripRatePerSec: float64(cfg.DripRateUgnotPerSec),
		dripTokens:     float64(cfg.DripBurstUgnot), // bucket starts full
		addrHits:       make(map[string][]time.Time),
		ipHits:         make(map[string][]time.Time),
	}
}

// Allow records a grant for addr/ip and returns the time it was recorded, or a
// non-nil error (ErrCooldown, ErrRateLimited, ErrDailyCap, ErrDripLimited) if
// blocked. Checks run in order: addr → IP → daily cap → global drip. The
// returned time is passed to Refund so a refund knows which UTC day the grant
// counted against.
func (l *Limiter) Allow(addr, ip string) (time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	// Reset daily counter on a new UTC calendar day, and sweep keys whose hits
	// have all aged out so the maps don't grow without bound over the service's
	// lifetime (an address/IP seen once and never again would otherwise persist).
	if l.dayStart.IsZero() || !sameUTCDay(l.dayStart, now) {
		l.daySpent = 0
		l.dayStart = now
		l.sweepStale(now)
	}

	// Check per-address window.
	if countWithin(l.addrHits[addr], now, l.perAddrWindow) >= l.perAddrMax {
		return time.Time{}, ErrCooldown
	}

	// Check per-IP window.
	if countWithin(l.ipHits[ip], now, l.perIPWindow) >= l.perIPMax {
		return time.Time{}, ErrRateLimited
	}

	// Check global daily cap. Written as a subtraction to avoid an int64 overflow
	// when an operator sets dailyCapUgnot near math.MaxInt64.
	if l.daySpent > l.dailyCapUgnot-l.grantUgnot {
		return time.Time{}, ErrDailyCap
	}

	// Check global drip token bucket (skipped when disabled).
	if l.dripBurst > 0 {
		l.refillDrip(now)
		if l.dripTokens < float64(l.grantUgnot) {
			return time.Time{}, ErrDripLimited
		}
		l.dripTokens -= float64(l.grantUgnot)
	}

	// Grant: prune stale hits, record this hit, update counter.
	l.addrHits[addr] = pruneAndAppend(l.addrHits[addr], now, l.perAddrWindow)
	l.ipHits[ip] = pruneAndAppend(l.ipHits[ip], now, l.perIPWindow)
	l.daySpent += l.grantUgnot

	return now, nil
}

// Refund reverses the accounting of the Allow at grantedAt for addr/ip: it drops
// their newest recorded hit and credits the grant back to the daily counter.
// Called when a dispense fails after Allow succeeded so a chain error doesn't
// consume the requester's cooldown or the global daily budget. The daily credit
// is skipped if a day-rollover has reset daySpent since the grant — that reset
// already discarded the grant's contribution, so crediting again would
// under-count the new day and let the cap be exceeded.
func (l *Limiter) Refund(addr, ip string, grantedAt time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.addrHits[addr] = dropLast(l.addrHits[addr])
	l.ipHits[ip] = dropLast(l.ipHits[ip])
	if sameUTCDay(grantedAt, l.dayStart) {
		l.daySpent -= l.grantUgnot
		if l.daySpent < 0 {
			l.daySpent = 0
		}
	}
	// Credit the grant back to the drip bucket so a failed dispense doesn't
	// permanently consume drip capacity. Refill first so the credit lands on a
	// current bucket, then cap at burst. Relies on Refund only ever following a
	// successful Allow, which has already seeded dripLast — so this never
	// credits a grant the bucket did not spend.
	if l.dripBurst > 0 {
		l.refillDrip(l.now())
		l.dripTokens += float64(l.grantUgnot)
		if l.dripTokens > l.dripBurst {
			l.dripTokens = l.dripBurst
		}
	}
}

// LimiterSnapshot is a point-in-time view of the limiter's global budget state,
// for metrics. It carries no per-address or per-IP data.
type LimiterSnapshot struct {
	DripEnabled   bool
	DripTokens    float64 // projected tokens available now (0 when disabled)
	DripBurst     float64 // bucket capacity
	DailyCapUgnot int64
	DaySpentUgnot int64 // 0 once the UTC day has rolled over
}

// Snapshot returns the current global budget state without mutating the limiter:
// it projects accrued drip tokens up to now (capped at burst) without advancing
// the bucket clock, and reports zero daily spend once the UTC day has rolled
// over. Safe to call on every scrape.
func (l *Limiter) Snapshot() LimiterSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	daySpent := l.daySpent
	if l.dayStart.IsZero() || !sameUTCDay(l.dayStart, now) {
		daySpent = 0
	}

	s := LimiterSnapshot{
		DripEnabled:   l.dripBurst > 0,
		DripBurst:     l.dripBurst,
		DailyCapUgnot: l.dailyCapUgnot,
		DaySpentUgnot: daySpent,
	}
	if l.dripBurst > 0 {
		tokens := l.dripTokens
		if !l.dripLast.IsZero() {
			if elapsed := now.Sub(l.dripLast).Seconds(); elapsed > 0 {
				tokens += elapsed * l.dripRatePerSec
				if tokens > l.dripBurst {
					tokens = l.dripBurst
				}
			}
		}
		s.DripTokens = tokens
	}
	return s
}

// refillDrip adds tokens accrued since the last refill, capped at burst.
// Callers must hold l.mu. The first call seeds dripLast without accruing.
func (l *Limiter) refillDrip(now time.Time) {
	if l.dripLast.IsZero() {
		l.dripLast = now
		return
	}
	elapsed := now.Sub(l.dripLast).Seconds()
	if elapsed <= 0 {
		return
	}
	l.dripTokens += elapsed * l.dripRatePerSec
	if l.dripTokens > l.dripBurst {
		l.dripTokens = l.dripBurst
	}
	l.dripLast = now
}

// sweepStale drops keys whose hits have all aged out of their window. Callers
// must hold l.mu.
func (l *Limiter) sweepStale(now time.Time) {
	for k, hits := range l.addrHits {
		if kept := prune(hits, now, l.perAddrWindow); len(kept) == 0 {
			delete(l.addrHits, k)
		} else {
			l.addrHits[k] = kept
		}
	}
	for k, hits := range l.ipHits {
		if kept := prune(hits, now, l.perIPWindow); len(kept) == 0 {
			delete(l.ipHits, k)
		} else {
			l.ipHits[k] = kept
		}
	}
}

// sameUTCDay reports whether a and b fall on the same UTC calendar day.
func sameUTCDay(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// countWithin returns the number of timestamps in hits that fall within
// [now-window, now].
func countWithin(hits []time.Time, now time.Time, window time.Duration) int {
	cutoff := now.Add(-window)
	count := 0
	for _, h := range hits {
		if !h.Before(cutoff) {
			count++
		}
	}
	return count
}

// prune removes timestamps outside [now-window, now] from hits in place and
// returns the resulting slice.
func prune(hits []time.Time, now time.Time, window time.Duration) []time.Time {
	cutoff := now.Add(-window)
	out := hits[:0]
	for _, h := range hits {
		if !h.Before(cutoff) {
			out = append(out, h)
		}
	}
	return out
}

// pruneAndAppend prunes stale timestamps from hits and appends now.
func pruneAndAppend(hits []time.Time, now time.Time, window time.Duration) []time.Time {
	return append(prune(hits, now, window), now)
}

// dropLast returns hits without its final element (the most recently appended).
func dropLast(hits []time.Time) []time.Time {
	if len(hits) == 0 {
		return hits
	}
	return hits[:len(hits)-1]
}
