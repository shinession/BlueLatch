package latch

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
)

// ErrGaveUp is returned when Policy.MaxAttempts attempts did not connect the device.
var ErrGaveUp = errors.New("gave up")

// Phase says what Engine.Connect is doing.
type Phase int

const (
	// PhaseReconnecting means Windows is being asked to connect the device. The
	// call can take several seconds and cannot be interrupted.
	PhaseReconnecting Phase = iota + 1

	// PhaseWaiting means Windows has answered and the link is awaited.
	PhaseWaiting

	// PhaseBackoff means the attempt failed and the next one waits for Progress.Delay.
	PhaseBackoff

	// PhaseConnected means the device is connected. It is the last report.
	PhaseConnected
)

func (p Phase) String() string {
	switch p {
	case PhaseReconnecting:
		return "reconnecting"
	case PhaseWaiting:
		return "waiting"
	case PhaseBackoff:
		return "backoff"
	case PhaseConnected:
		return "connected"
	}
	return fmt.Sprintf("phase(%d)", int(p))
}

// Progress is what Connect reports while it works.
type Progress struct {
	Phase   Phase
	Attempt int           // the attempt in progress, from 1; 0 when the device was connected already
	Elapsed time.Duration // since Connect was called
	Device  bt.Device     // the latest state read from Windows
	Detail  string        // what Windows did with each service in this attempt (PhaseWaiting, PhaseBackoff)
	Err     error         // the error Windows reported for this attempt, if any (PhaseWaiting, PhaseBackoff)
	Delay   time.Duration // the pause before the next attempt (PhaseBackoff)
}

// Result summarises a successful Connect.
type Result struct {
	Attempts int
	Elapsed  time.Duration
}

// Engine connects one device at a time.
type Engine struct {
	adapter bt.Adapter
	policy  Policy
	log     *slog.Logger

	// The clock is replaced in tests.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

// New returns an Engine. A nil logger discards its log lines.
func New(adapter bt.Adapter, policy Policy, log *slog.Logger) *Engine {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Engine{
		adapter: adapter,
		policy:  policy.normalized(),
		log:     log,
		now:     time.Now,
		sleep:   sleepContext,
	}
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// isFatal reports whether retrying cannot help: there is no radio, or Windows
// holds no pairing or no audio service for the device.
func isFatal(err error) bool {
	return errors.Is(err, bt.ErrNoRadio) ||
		errors.Is(err, bt.ErrDeviceNotFound) ||
		errors.Is(err, bt.ErrNoAudioServices)
}

// Connect asks Windows to connect the device, again and again, until the device
// has stayed connected for Policy.StableFor. It returns when that happens, when
// ctx is done (the error is then ctx.Err()), when Policy.MaxAttempts attempts
// failed (ErrGaveUp), or when retrying cannot help (the error wraps bt.ErrNoRadio,
// bt.ErrDeviceNotFound or bt.ErrNoAudioServices). A device that is connected
// already is reported as connected at once and is not touched.
//
// report, which may be nil, is called from the goroutine that called Connect.
//
// Cancelling ctx does not interrupt a reconnect request that Windows is working
// on: that request switches services off and on again and must run to its end.
// Connect returns as soon as it has.
func (e *Engine) Connect(ctx context.Context, addr bt.Address, report func(Progress)) (Result, error) {
	start := e.now()
	attempts := 0
	var dev bt.Device

	emit := func(p Progress) {
		p.Attempt = attempts
		p.Elapsed = e.now().Sub(start)
		p.Device = dev
		if report != nil {
			report(p)
		}
	}
	result := func() Result { return Result{Attempts: attempts, Elapsed: e.now().Sub(start)} }

	dev, err := e.adapter.Device(addr)
	if err != nil {
		return result(), fmt.Errorf("reading the device: %w", err)
	}
	log := e.log.With("device", dev.DisplayName(), "address", addr.String())
	if dev.Connected {
		log.Info("the device is connected already")
		emit(Progress{Phase: PhaseConnected})
		return result(), nil
	}
	log.Info("connecting", "attempt_timeout", e.policy.AttemptTimeout, "stable_for", e.policy.StableFor, "max_attempts", e.policy.MaxAttempts)

	for {
		if attempts > 0 {
			// Windows may have connected the device by itself while this loop
			// paused. Do not disturb a link that is coming up.
			ok, err := e.waitLink(ctx, addr, &dev, e.now())
			if err != nil {
				return result(), err
			}
			if ok {
				return e.connected(log, emit, result()), nil
			}
		}

		attempts++
		emit(Progress{Phase: PhaseReconnecting})
		reconnected, rerr := e.adapter.Reconnect(addr)
		detail := reconnected.String()
		if rerr != nil {
			log.Warn("the reconnect request failed", "attempt", attempts, "err", rerr)
		} else {
			log.Info("the reconnect request was answered", "attempt", attempts, "services", detail)
		}
		if isFatal(rerr) {
			return result(), rerr
		}
		if err := ctx.Err(); err != nil {
			return result(), err
		}

		emit(Progress{Phase: PhaseWaiting, Detail: detail, Err: rerr})
		ok, err := e.waitLink(ctx, addr, &dev, e.now().Add(e.policy.AttemptTimeout))
		if err != nil {
			return result(), err
		}
		if ok {
			return e.connected(log, emit, result()), nil
		}

		if limit := e.policy.MaxAttempts; limit > 0 && attempts >= limit {
			log.Warn("giving up", "attempts", attempts)
			return result(), fmt.Errorf("%w after %d attempts", ErrGaveUp, attempts)
		}
		delay := e.policy.backoff(attempts)
		log.Info("the device did not connect", "attempt", attempts, "next_attempt_in", delay)
		emit(Progress{Phase: PhaseBackoff, Detail: detail, Err: rerr, Delay: delay})
		if err := e.sleep(ctx, delay); err != nil {
			return result(), err
		}
	}
}

func (e *Engine) connected(log *slog.Logger, emit func(Progress), res Result) Result {
	log.Info("connected", "attempts", res.Attempts, "elapsed", res.Elapsed)
	emit(Progress{Phase: PhaseConnected})
	return res
}

// waitLink reads the device until it has been connected for Policy.StableFor,
// and reports true then. It reports false once deadline has passed without a
// connection in progress. A connection that is up at the deadline is given the
// time to prove itself. dev is updated with every successful read.
func (e *Engine) waitLink(ctx context.Context, addr bt.Address, dev *bt.Device, deadline time.Time) (bool, error) {
	var since time.Time // when the current run of "connected" readings began
	for {
		latest, err := e.adapter.Device(addr)
		connected := false
		switch {
		case err == nil:
			*dev = latest
			connected = latest.Connected
		case isFatal(err):
			return false, fmt.Errorf("reading the device: %w", err)
		default:
			e.log.Warn("reading the device failed, counting it as not connected", "err", err)
		}
		e.log.Debug("poll", "connected", connected)

		now := e.now()
		if connected {
			if since.IsZero() {
				since = now
			}
			if now.Sub(since) >= e.policy.StableFor {
				return true, nil
			}
		} else {
			since = time.Time{}
			if !now.Before(deadline) {
				return false, nil
			}
		}
		if err := e.sleep(ctx, e.policy.PollInterval); err != nil {
			return false, err
		}
	}
}
