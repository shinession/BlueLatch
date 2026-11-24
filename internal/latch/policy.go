// Package latch is the retry engine: it asks Windows to connect a device again
// and again until the connection holds, the user cancels, or something happens
// that retrying cannot fix. It knows nothing about Windows; it works through
// bt.Adapter, so it is tested with a fake.
package latch

import "time"

// Policy tunes how Engine.Connect retries.
type Policy struct {
	// AttemptTimeout is how long one attempt waits for the link after Windows
	// has answered the reconnect request.
	AttemptTimeout time.Duration

	// PollInterval is how often the device state is read while waiting.
	PollInterval time.Duration

	// StableFor is how long the device must stay connected before it counts as
	// connected. It filters out a link that comes up and drops again at once.
	StableFor time.Duration

	// BackoffStart is the pause after the first failed attempt. Each following
	// pause is half as long again, up to BackoffMax.
	BackoffStart time.Duration
	BackoffMax   time.Duration

	// MaxAttempts stops the retrying after that many attempts. Zero means no limit.
	MaxAttempts int
}

// DefaultPolicy returns the retry settings BlueLatch uses unless told otherwise.
func DefaultPolicy() Policy {
	return Policy{
		AttemptTimeout: 10 * time.Second,
		PollInterval:   500 * time.Millisecond,
		StableFor:      2 * time.Second,
		BackoffStart:   2 * time.Second,
		BackoffMax:     5 * time.Second,
	}
}

// normalized replaces the values that would break the loop (a zero poll
// interval, for instance) with the defaults. StableFor and MaxAttempts may be
// zero on purpose.
func (p Policy) normalized() Policy {
	d := DefaultPolicy()
	if p.AttemptTimeout <= 0 {
		p.AttemptTimeout = d.AttemptTimeout
	}
	if p.PollInterval <= 0 {
		p.PollInterval = d.PollInterval
	}
	if p.StableFor < 0 {
		p.StableFor = d.StableFor
	}
	if p.BackoffStart <= 0 {
		p.BackoffStart = d.BackoffStart
	}
	if p.BackoffMax <= 0 {
		p.BackoffMax = d.BackoffMax
	}
	if p.BackoffMax < p.BackoffStart {
		p.BackoffMax = p.BackoffStart
	}
	if p.MaxAttempts < 0 {
		p.MaxAttempts = 0
	}
	return p
}

// backoff returns the pause after the given failed attempt (1 for the first).
func (p Policy) backoff(attempt int) time.Duration {
	d := p.BackoffStart
	for i := 1; i < attempt; i++ {
		d = d * 3 / 2
		if d >= p.BackoffMax {
			return p.BackoffMax
		}
	}
	return min(d, p.BackoffMax)
}
