package session

import (
	"errors"
	"fmt"
)

// ErrNoActiveSession is returned when no session exists for the profile that
// is active, unexpired, and has remaining spend — regardless of realm.
var ErrNoActiveSession = errors.New("session: no active session for profile")

// ErrScopeMismatch is returned when at least one active session exists for the
// profile but none covers the requested realm.
type ErrScopeMismatch struct {
	AvailablePaths []string
}

func (e *ErrScopeMismatch) Error() string {
	return fmt.Sprintf(
		"session: realm not in any active session's AllowPaths (available: %v)",
		e.AvailablePaths,
	)
}
