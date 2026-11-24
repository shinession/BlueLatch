package latch

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
)

// fakeClock is a clock that only moves when the code under test sleeps or the
// fake adapter says a Windows call took time, so the tests run instantly.
type fakeClock struct{ t time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time { return c.t }

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.t = c.t.Add(d)
	return nil
}

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// span is a period during which the fake device is connected. A zero to means
// until the end of the test.
type span struct{ from, to time.Time }

type fakeAdapter struct {
	clock      *fakeClock
	device     bt.Device
	up         []span
	reconnects int
	polls      int

	// onReconnect runs inside Reconnect with the number of the call, from 1. It
	// may advance the clock and add spans.
	onReconnect func(a *fakeAdapter, n int) (bt.ReconnectResult, error)

	// deviceErr, when set, is asked before every read of the device.
	deviceErr func(a *fakeAdapter) error
}

func (a *fakeAdapter) Devices() ([]bt.Device, error) { return []bt.Device{a.current()}, nil }

func (a *fakeAdapter) Device(bt.Address) (bt.Device, error) {
	a.polls++
	if a.deviceErr != nil {
		if err := a.deviceErr(a); err != nil {
			return bt.Device{}, err
		}
	}
	return a.current(), nil
}

func (a *fakeAdapter) Reconnect(bt.Address) (bt.ReconnectResult, error) {
	a.reconnects++
	if a.onReconnect == nil {
		return bt.ReconnectResult{}, nil
	}
	return a.onReconnect(a, a.reconnects)
}

func (a *fakeAdapter) current() bt.Device {
	d := a.device
	now := a.clock.Now()
	for _, s := range a.up {
		if !now.Before(s.from) && (s.to.IsZero() || now.Before(s.to)) {
			d.Connected = true
		}
	}
	return d
}

func newFake() *fakeAdapter {
	return &fakeAdapter{clock: newFakeClock(), device: bt.Device{Name: "Buds", Address: 1}}
}

func (a *fakeAdapter) engine(p Policy) *Engine {
	e := New(a, p, nil)
	e.now = a.clock.Now
	e.sleep = a.clock.Sleep
	return e
}

// phases returns the phase of every report, in order.
func phases(reports []Progress) []Phase {
	var out []Phase
	for _, p := range reports {
		out = append(out, p.Phase)
	}
	return out
}

func collect(reports *[]Progress) func(Progress) {
	return func(p Progress) { *reports = append(*reports, p) }
}

func TestConnectLeavesAConnectedDeviceAlone(t *testing.T) {
	a := newFake()
	a.up = []span{{from: a.clock.Now()}}
	var reports []Progress

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, collect(&reports))

	if err != nil || res.Attempts != 0 || res.Elapsed != 0 {
		t.Fatalf("Connect = %+v, %v; want no attempts and no error", res, err)
	}
	if a.reconnects != 0 {
		t.Errorf("Reconnect was called %d times for a device that was connected already", a.reconnects)
	}
	if got := phases(reports); !slices.Equal(got, []Phase{PhaseConnected}) {
		t.Errorf("phases = %v, want [connected]", got)
	}
}

func TestConnectSucceedsOnTheThirdAttempt(t *testing.T) {
	a := newFake()
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		a.clock.advance(time.Second) // the Windows call takes a second
		if n == 3 {
			a.up = append(a.up, span{from: a.clock.Now().Add(1500 * time.Millisecond)})
		}
		return bt.ReconnectResult{}, nil
	}
	var reports []Progress

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, collect(&reports))

	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if res.Attempts != 3 || a.reconnects != 3 {
		t.Errorf("attempts = %d, Reconnect calls = %d; want 3 and 3", res.Attempts, a.reconnects)
	}
	// Attempt 3 ends at 28s; the link comes up at 29.5s and must hold for 2s.
	if want := 31500 * time.Millisecond; res.Elapsed != want {
		t.Errorf("elapsed = %v, want %v", res.Elapsed, want)
	}
	wantPhases := []Phase{
		PhaseReconnecting, PhaseWaiting, PhaseBackoff,
		PhaseReconnecting, PhaseWaiting, PhaseBackoff,
		PhaseReconnecting, PhaseWaiting, PhaseConnected,
	}
	if got := phases(reports); !slices.Equal(got, wantPhases) {
		t.Errorf("phases = %v, want %v", got, wantPhases)
	}
	var delays []time.Duration
	for _, p := range reports {
		if p.Phase == PhaseBackoff {
			delays = append(delays, p.Delay)
		}
	}
	if want := []time.Duration{2 * time.Second, 3 * time.Second}; !slices.Equal(delays, want) {
		t.Errorf("pauses = %v, want %v", delays, want)
	}
	if last := reports[len(reports)-1]; last.Attempt != 3 || !last.Device.Connected {
		t.Errorf("last report = %+v, want attempt 3 with a connected device", last)
	}
}

func TestConnectWaitsUntilTheLinkHolds(t *testing.T) {
	a := newFake()
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		a.clock.advance(time.Second)
		now := a.clock.Now()
		a.up = []span{
			{from: now.Add(1 * time.Second), to: now.Add(2 * time.Second)}, // one second: too short
			{from: now.Add(5 * time.Second)},
		}
		return bt.ReconnectResult{}, nil
	}

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	// The call ends at 1s, the second span starts at 6s and holds for 2s.
	if res.Attempts != 1 || res.Elapsed != 8*time.Second {
		t.Errorf("attempts = %d, elapsed = %v; want 1 and 8s", res.Attempts, res.Elapsed)
	}
}

func TestConnectDoesNotDisturbALinkThatComesUpByItself(t *testing.T) {
	a := newFake()
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		a.clock.advance(time.Second)
		return bt.ReconnectResult{}, nil
	}
	// Attempt 1 gives up waiting at 11s and pauses until 13s. The device comes
	// up at 12.5s, during the pause.
	a.up = []span{{from: a.clock.Now().Add(12500 * time.Millisecond)}}

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if a.reconnects != 1 || res.Attempts != 1 {
		t.Errorf("Reconnect calls = %d, attempts = %d; want 1 and 1 (no second request)", a.reconnects, res.Attempts)
	}
	if want := 15 * time.Second; res.Elapsed != want {
		t.Errorf("elapsed = %v, want %v", res.Elapsed, want)
	}
}

func TestConnectKeepsGoingAfterTheRequestFails(t *testing.T) {
	a := newFake()
	boom := errors.New("boom")
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		a.clock.advance(time.Second)
		if n <= 2 {
			res := bt.ReconnectResult{Services: []bt.ServiceResult{{Service: bt.ServiceA2DPSink, EnableErr: boom}}}
			return res, errors.New("no audio service could be enabled: " + res.String())
		}
		a.up = append(a.up, span{from: a.clock.Now()})
		return bt.ReconnectResult{Services: []bt.ServiceResult{{Service: bt.ServiceA2DPSink}}}, nil
	}
	var reports []Progress

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, collect(&reports))

	if err != nil || res.Attempts != 3 {
		t.Fatalf("Connect = %+v, %v; want success on attempt 3", res, err)
	}
	var failed int
	for _, p := range reports {
		if p.Phase != PhaseBackoff {
			continue
		}
		failed++
		if p.Err == nil || !strings.Contains(p.Detail, "A2DP Sink: enable failed: boom") {
			t.Errorf("backoff report of attempt %d = err %v, detail %q; want the failure", p.Attempt, p.Err, p.Detail)
		}
	}
	if failed != 2 {
		t.Errorf("%d backoff reports, want 2", failed)
	}
}

func TestConnectStopsWhenThePolicyAllowsNoMoreAttempts(t *testing.T) {
	a := newFake()
	policy := DefaultPolicy()
	policy.MaxAttempts = 2
	var reports []Progress

	res, err := a.engine(policy).Connect(context.Background(), 1, collect(&reports))

	if !errors.Is(err, ErrGaveUp) {
		t.Fatalf("error = %v, want ErrGaveUp", err)
	}
	if res.Attempts != 2 || a.reconnects != 2 {
		t.Errorf("attempts = %d, Reconnect calls = %d; want 2 and 2", res.Attempts, a.reconnects)
	}
	// 2 attempts of 10s plus the single pause of 2s between them: no pause after the last one.
	if want := 22 * time.Second; res.Elapsed != want {
		t.Errorf("elapsed = %v, want %v", res.Elapsed, want)
	}
	if got := phases(reports); got[len(got)-1] == PhaseBackoff {
		t.Errorf("the last report is a pause: %v", got)
	}
}

func TestConnectStopsWhenCancelledDuringAPause(t *testing.T) {
	a := newFake()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res, err := a.engine(DefaultPolicy()).Connect(ctx, 1, func(p Progress) {
		if p.Phase == PhaseBackoff {
			cancel()
		}
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if res.Attempts != 1 || a.reconnects != 1 {
		t.Errorf("attempts = %d, Reconnect calls = %d; want 1 and 1", res.Attempts, a.reconnects)
	}
}

func TestConnectLetsARequestInProgressFinishWhenCancelled(t *testing.T) {
	a := newFake()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := false
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		cancel() // the user stops while Windows is still working
		a.clock.advance(3 * time.Second)
		finished = true
		return bt.ReconnectResult{}, nil
	}

	res, err := a.engine(DefaultPolicy()).Connect(ctx, 1, nil)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if !finished {
		t.Error("Connect returned before the reconnect request had finished")
	}
	if a.polls != 1 {
		t.Errorf("the device was read %d times, want only the first read before the request", a.polls)
	}
	if res.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", res.Attempts)
	}
}

func TestConnectStopsWhenTheDeviceIsGone(t *testing.T) {
	a := newFake()
	a.deviceErr = func(a *fakeAdapter) error {
		if a.reconnects >= 1 {
			return bt.ErrDeviceNotFound
		}
		return nil
	}

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if !errors.Is(err, bt.ErrDeviceNotFound) {
		t.Fatalf("error = %v, want bt.ErrDeviceNotFound", err)
	}
	if res.Attempts != 1 {
		t.Errorf("attempts = %d, want 1", res.Attempts)
	}
}

func TestConnectStopsWhenThereIsNoRadio(t *testing.T) {
	a := newFake()
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		return bt.ReconnectResult{}, bt.ErrNoRadio
	}
	start := a.clock.Now()

	_, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if !errors.Is(err, bt.ErrNoRadio) {
		t.Fatalf("error = %v, want bt.ErrNoRadio", err)
	}
	if a.reconnects != 1 || !a.clock.Now().Equal(start) {
		t.Errorf("Reconnect calls = %d, time passed = %v; want 1 call and no waiting", a.reconnects, a.clock.Now().Sub(start))
	}
}

func TestConnectStopsWhenTheDeviceHasNoAudioServices(t *testing.T) {
	a := newFake()
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		return bt.ReconnectResult{}, bt.ErrNoAudioServices
	}

	_, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if !errors.Is(err, bt.ErrNoAudioServices) || a.reconnects != 1 {
		t.Errorf("error = %v after %d calls; want bt.ErrNoAudioServices after 1", err, a.reconnects)
	}
}

func TestConnectFailsWhenTheDeviceCannotBeRead(t *testing.T) {
	a := newFake()
	a.deviceErr = func(*fakeAdapter) error { return bt.ErrNoRadio }

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if !errors.Is(err, bt.ErrNoRadio) {
		t.Fatalf("error = %v, want bt.ErrNoRadio", err)
	}
	if res.Attempts != 0 || a.reconnects != 0 {
		t.Errorf("attempts = %d, Reconnect calls = %d; want none", res.Attempts, a.reconnects)
	}
}

func TestConnectCountsATransientReadErrorAsNotConnected(t *testing.T) {
	a := newFake()
	transient := errors.New("transient")
	a.onReconnect = func(a *fakeAdapter, n int) (bt.ReconnectResult, error) {
		a.up = append(a.up, span{from: a.clock.Now()})
		return bt.ReconnectResult{}, nil
	}
	a.deviceErr = func(a *fakeAdapter) error {
		if a.reconnects == 1 && a.polls == 2 { // the first read after the request fails
			return transient
		}
		return nil
	}

	res, err := a.engine(DefaultPolicy()).Connect(context.Background(), 1, nil)

	if err != nil || res.Attempts != 1 {
		t.Fatalf("Connect = %+v, %v; want success on attempt 1", res, err)
	}
	// One read failed at 0s, the link counts from the next read at 0.5s and holds 2s.
	if want := 2500 * time.Millisecond; res.Elapsed != want {
		t.Errorf("elapsed = %v, want %v", res.Elapsed, want)
	}
}
