package latch

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	p := DefaultPolicy()
	want := []time.Duration{
		2 * time.Second,
		3 * time.Second,
		4500 * time.Millisecond,
		5 * time.Second,
		5 * time.Second,
		5 * time.Second,
	}
	for i, w := range want {
		if got := p.backoff(i + 1); got != w {
			t.Errorf("backoff(%d) = %v, want %v", i+1, got, w)
		}
	}
}

func TestNormalizedFillsInWhatWouldBreakTheLoop(t *testing.T) {
	got := Policy{}.normalized()
	d := DefaultPolicy()
	if got.AttemptTimeout != d.AttemptTimeout || got.PollInterval != d.PollInterval ||
		got.BackoffStart != d.BackoffStart || got.BackoffMax != d.BackoffMax ||
		got.DisconnectTimeout != d.DisconnectTimeout {
		t.Errorf("Policy{}.normalized() = %+v, want the defaults for the timings", got)
	}
	if got.StableFor != 0 || got.MaxAttempts != 0 {
		t.Errorf("StableFor = %v, MaxAttempts = %d; zero is a valid choice for both", got.StableFor, got.MaxAttempts)
	}

	got = Policy{StableFor: -1, MaxAttempts: -3, BackoffStart: 4 * time.Second, BackoffMax: time.Second}.normalized()
	if got.StableFor != d.StableFor || got.MaxAttempts != 0 {
		t.Errorf("negative values: StableFor = %v, MaxAttempts = %d", got.StableFor, got.MaxAttempts)
	}
	if got.BackoffMax != got.BackoffStart {
		t.Errorf("BackoffMax = %v, want it raised to BackoffStart (%v)", got.BackoffMax, got.BackoffStart)
	}
}
