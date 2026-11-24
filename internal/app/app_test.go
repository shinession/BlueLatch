package app

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
)

const (
	addrSpeaker bt.Address = 0xAABBCCDDEE01
	addrBuds    bt.Address = 0xAABBCCDDEE02
	addrMouse   bt.Address = 0xAABBCCDDEE03
)

// fakeAdapter has a speaker that is connected, earbuds that connect after a
// number of requests, and a mouse.
type fakeAdapter struct {
	mu           sync.Mutex
	devicesErr   error
	reconnectErr error
	connectAfter int           // the number of completed requests after which the earbuds connect; 0 means never
	gate         chan struct{} // when set, Reconnect waits until it is closed
	entered      chan struct{} // when set, Reconnect signals here when it has started
	completed    int
}

func (f *fakeAdapter) list() []bt.Device {
	buds := bt.Device{Name: "Example Buds", Address: addrBuds, Class: 0x248404, Remembered: true, Authenticated: true,
		Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceHandsfree}}
	buds.Connected = f.connectAfter > 0 && f.completed >= f.connectAfter
	return []bt.Device{
		buds,
		{Name: "Example Mouse", Address: addrMouse, Class: 0x002580, Remembered: true, Authenticated: true},
		{Name: "Example Speaker", Address: addrSpeaker, Class: 0x240414, Connected: true, Remembered: true, Authenticated: true,
			Services: []bt.Service{bt.ServiceA2DPSink}},
	}
}

func (f *fakeAdapter) Devices() ([]bt.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.devicesErr != nil {
		return nil, f.devicesErr
	}
	return f.list(), nil
}

func (f *fakeAdapter) Device(addr bt.Address) (bt.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, d := range f.list() {
		if d.Address == addr {
			return d, nil
		}
	}
	return bt.Device{}, bt.ErrDeviceNotFound
}

func (f *fakeAdapter) Reconnect(bt.Address) (bt.ReconnectResult, error) {
	f.mu.Lock()
	gate, entered, err := f.gate, f.entered, f.reconnectErr
	f.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		<-gate
	}
	f.mu.Lock()
	f.completed++
	f.mu.Unlock()
	return bt.ReconnectResult{Services: []bt.ServiceResult{{Service: bt.ServiceA2DPSink}}}, err
}

func (f *fakeAdapter) set(fn func(*fakeAdapter)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func fastPolicy() latch.Policy {
	return latch.Policy{
		AttemptTimeout: 60 * time.Millisecond,
		PollInterval:   5 * time.Millisecond,
		StableFor:      10 * time.Millisecond,
		BackoffStart:   5 * time.Millisecond,
		BackoffMax:     10 * time.Millisecond,
	}
}

func newTestApp(t *testing.T, adapter bt.Adapter) *App {
	t.Helper()
	a := New(Config{Adapter: adapter, Policy: fastPolicy(), Refresh: 10 * time.Millisecond, Version: "test"})
	a.Start()
	t.Cleanup(func() { a.Shutdown(5 * time.Second) })
	return a
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func jobDone(a *App) bool {
	j := a.Snapshot().Job
	return j != nil && j.Done
}

func TestSnapshotListsThePairedAudioDevices(t *testing.T) {
	a := newTestApp(t, &fakeAdapter{})

	s := a.Snapshot()

	if s.Version != "test" || s.Problem != nil || s.Job != nil || s.Closing {
		t.Errorf("snapshot = %+v, want a quiet state", s)
	}
	want := []DeviceView{
		{Name: "Example Buds", Address: "AA:BB:CC:DD:EE:02", Kind: "headset", Connected: false},
		{Name: "Example Speaker", Address: "AA:BB:CC:DD:EE:01", Kind: "speaker", Connected: true},
	}
	if !slices.Equal(s.Devices, want) {
		t.Errorf("devices = %+v, want %+v (the mouse is not an audio device)", s.Devices, want)
	}
}

func TestConnectRetriesUntilTheDeviceConnects(t *testing.T) {
	adapter := &fakeAdapter{connectAfter: 2}
	a := newTestApp(t, adapter)

	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if got := a.Snapshot().Job; got == nil || got.Name != "Example Buds" || got.Done {
		t.Errorf("job right after Connect = %+v, want a running job for the earbuds", got)
	}
	waitFor(t, "the job to end", func() bool { return jobDone(a) })

	job := a.Snapshot().Job
	if job.Phase != PhaseConnected || job.Attempt != 2 || job.Error != "" || job.DoneAt == 0 {
		t.Errorf("job = %+v, want connected after 2 attempts", job)
	}
	waitFor(t, "the list to show the earbuds as connected", func() bool {
		for _, d := range a.Snapshot().Devices {
			if d.Address == "AA:BB:CC:DD:EE:02" {
				return d.Connected
			}
		}
		return false
	})
	if a.Busy() {
		t.Error("Busy() is true after the job ended")
	}
	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Errorf("a new Connect after the job ended: %v", err)
	}
}

func TestConnectRefusesASecondJob(t *testing.T) {
	adapter := &fakeAdapter{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	a := newTestApp(t, adapter)

	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	<-adapter.entered
	if err := a.Connect("AA:BB:CC:DD:EE:01"); !errors.Is(err, ErrBusy) {
		t.Errorf("a second Connect = %v, want ErrBusy", err)
	}
	if !a.Busy() {
		t.Error("Busy() is false while a request is in progress")
	}
	close(adapter.gate)
}

func TestConnectRefusesWhatIsNotInTheList(t *testing.T) {
	a := newTestApp(t, &fakeAdapter{})

	if err := a.Connect("not an address"); err == nil {
		t.Error("Connect accepted text that is not an address")
	}
	for _, addr := range []string{"00:00:00:00:00:99", "AA:BB:CC:DD:EE:03"} { // unknown, and the mouse
		if err := a.Connect(addr); !errors.Is(err, ErrUnknownDevice) {
			t.Errorf("Connect(%s) = %v, want ErrUnknownDevice", addr, err)
		}
	}
	if a.Snapshot().Job != nil {
		t.Error("a refused Connect left a job behind")
	}
}

func TestCancelStopsTheJob(t *testing.T) {
	a := newTestApp(t, &fakeAdapter{}) // the earbuds never connect

	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the first attempt to fail", func() bool {
		j := a.Snapshot().Job
		return j != nil && j.Phase == PhaseBackoff && j.NextAt > 0 && j.Attempt >= 1
	})
	a.Cancel()
	waitFor(t, "the job to end", func() bool { return jobDone(a) })

	if got := a.Snapshot().Job; got.Phase != PhaseCancelled || got.Error != "" {
		t.Errorf("job = %+v, want cancelled", got)
	}
}

func TestCancelDuringARequestWaitsForTheRequest(t *testing.T) {
	adapter := &fakeAdapter{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	a := newTestApp(t, adapter)
	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatal(err)
	}
	<-adapter.entered

	a.Cancel()

	if got := a.Snapshot().Job; got.Phase != PhaseCancelling || got.Done {
		t.Errorf("job = %+v, want it to be cancelling until the request returns", got)
	}
	close(adapter.gate)
	waitFor(t, "the job to end", func() bool { return jobDone(a) })
	if got := a.Snapshot().Job; got.Phase != PhaseCancelled {
		t.Errorf("job = %+v, want cancelled", got)
	}
}

func TestShutdownWaitsForARequestInProgress(t *testing.T) {
	adapter := &fakeAdapter{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	a := newTestApp(t, adapter)
	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatal(err)
	}
	<-adapter.entered

	result := make(chan bool, 1)
	go func() { result <- a.Shutdown(5 * time.Second) }()

	select {
	case <-result:
		t.Fatal("Shutdown returned while Windows was still working on the request")
	case <-time.After(80 * time.Millisecond):
	}
	if !a.Snapshot().Closing {
		t.Error("the state does not say that the program is closing")
	}
	close(adapter.gate)
	select {
	case ok := <-result:
		if !ok {
			t.Error("Shutdown reported a timeout although the request returned")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not return after the request returned")
	}
	if err := a.Connect("AA:BB:CC:DD:EE:02"); !errors.Is(err, ErrClosing) {
		t.Errorf("Connect after Shutdown = %v, want ErrClosing", err)
	}
}

func TestShutdownGivesUpAfterTheTimeout(t *testing.T) {
	adapter := &fakeAdapter{gate: make(chan struct{}), entered: make(chan struct{}, 8)}
	a := newTestApp(t, adapter)
	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatal(err)
	}
	<-adapter.entered

	if a.Shutdown(30 * time.Millisecond) {
		t.Error("Shutdown reported success although the request is still running")
	}
	close(adapter.gate) // let the goroutines end
}

func TestShutdownWithoutAJobReturnsAtOnce(t *testing.T) {
	a := New(Config{Adapter: &fakeAdapter{}, Refresh: 10 * time.Millisecond})
	a.Start()
	if !a.Shutdown(time.Second) || !a.Shutdown(time.Second) {
		t.Error("Shutdown did not return cleanly, or not twice")
	}
}

func TestProblemsAreShownAndClear(t *testing.T) {
	adapter := &fakeAdapter{devicesErr: bt.ErrNoRadio}
	a := newTestApp(t, adapter)

	if p := a.Snapshot().Problem; p == nil || p.Code != "no-radio" || len(a.Snapshot().Devices) != 0 {
		t.Fatalf("snapshot = %+v, want the no-radio problem and no devices", a.Snapshot())
	}

	adapter.set(func(f *fakeAdapter) { f.devicesErr = errors.New("boom") })
	waitFor(t, "the other problem", func() bool {
		p := a.Snapshot().Problem
		return p != nil && p.Code == "error" && p.Text == "boom"
	})

	adapter.set(func(f *fakeAdapter) { f.devicesErr = nil })
	waitFor(t, "the problem to clear", func() bool {
		s := a.Snapshot()
		return s.Problem == nil && len(s.Devices) == 2
	})
}

func TestAFailedJobSaysWhy(t *testing.T) {
	a := newTestApp(t, &fakeAdapter{reconnectErr: bt.ErrNoAudioServices})

	if err := a.Connect("AA:BB:CC:DD:EE:02"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the job to end", func() bool { return jobDone(a) })

	got := a.Snapshot().Job
	if got.Phase != PhaseFailed || got.ErrorCode != "no-services" || got.Error == "" {
		t.Errorf("job = %+v, want failed with the no-services code", got)
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{bt.ErrNoRadio, "no-radio"},
		{bt.ErrDeviceNotFound, "not-paired"},
		{bt.ErrNoAudioServices, "no-services"},
		{latch.ErrGaveUp, "gave-up"},
		{errors.New("other"), ""},
	}
	for _, tt := range tests {
		if got := errorCode(tt.err); got != tt.want {
			t.Errorf("errorCode(%v) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		class bt.ClassOfDevice
		want  string
	}{
		{0x240404, "headset"},
		{0x240408, "headset"}, // hands-free
		{0x240418, "headphones"},
		{0x240414, "speaker"},
		{0x240428, "speaker"}, // hi-fi audio
		{0x240420, "audio"},   // car audio
		{0x002580, "audio"},   // not audio/video at all
	}
	for _, tt := range tests {
		if got := kindOf(tt.class); got != tt.want {
			t.Errorf("kindOf(%#06x) = %q, want %q", uint32(tt.class), got, tt.want)
		}
	}
}

func TestDemoAdapter(t *testing.T) {
	d := newDemo(time.Millisecond)

	devices, err := d.Devices()
	if err != nil || len(devices) != 3 {
		t.Fatalf("Devices = %v, %v; want three devices", devices, err)
	}
	if !devices[2].Connected || devices[2].Name != "Demo Speaker" {
		t.Errorf("devices = %+v, want the speaker, sorted last, to be connected", devices)
	}

	for i := 1; i <= 3; i++ {
		res, err := d.Reconnect(DemoEarbuds)
		if err != nil || len(res.Services) != 2 {
			t.Fatalf("Reconnect %d = %v, %v; want both services", i, res, err)
		}
		if got, _ := d.Device(DemoEarbuds); got.Connected && i < 3 {
			t.Errorf("the earbuds connected after request %d, want request 3", i)
		}
	}
	waitFor(t, "the earbuds to connect", func() bool {
		got, _ := d.Device(DemoEarbuds)
		return got.Connected
	})

	for i := 0; i < 5; i++ {
		if _, err := d.Reconnect(DemoHeadphones); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(5 * time.Millisecond)
	if got, _ := d.Device(DemoHeadphones); got.Connected {
		t.Error("the headphones connected, want them never to")
	}
	if _, err := d.Device(0x1); !errors.Is(err, bt.ErrDeviceNotFound) {
		t.Errorf("Device(unknown) = %v, want ErrDeviceNotFound", err)
	}
	if _, err := d.Reconnect(0x1); !errors.Is(err, bt.ErrDeviceNotFound) {
		t.Errorf("Reconnect(unknown) = %v, want ErrDeviceNotFound", err)
	}
}
