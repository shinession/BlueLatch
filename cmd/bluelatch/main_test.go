package main

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
)

// staticAdapter reports fixed devices and fails any attempt to reconnect, so a
// test that reaches Reconnect by mistake fails.
type staticAdapter struct {
	devices []bt.Device
	err     error
}

func (s staticAdapter) Devices() ([]bt.Device, error) { return s.devices, s.err }

func (s staticAdapter) Device(addr bt.Address) (bt.Device, error) {
	for _, d := range s.devices {
		if d.Address == addr {
			return d, nil
		}
	}
	return bt.Device{}, bt.ErrDeviceNotFound
}

func (s staticAdapter) Reconnect(bt.Address) (bt.ReconnectResult, error) {
	return bt.ReconnectResult{}, errors.New("Reconnect was not expected")
}

// recoveringAdapter also implements bt.Recoverer.
type recoveringAdapter struct {
	staticAdapter
	restored []bt.Restored
	err      error
	called   *bool
}

func (r recoveringAdapter) Recover() ([]bt.Restored, error) {
	*r.called = true
	return r.restored, r.err
}

var testDevices = []bt.Device{
	{Name: "Example Speaker", Address: 0xAABBCCDDEE01, Class: 0x240414, Connected: true, Remembered: true, Authenticated: true,
		Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceAVRCPTarget}},
	{Name: "Example Buds", Address: 0xAABBCCDDEE02, Class: 0x248404, Remembered: true, Authenticated: true,
		Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceHandsfree}},
	{Name: "Example Mouse", Address: 0xAABBCCDDEE03, Class: 0x002580, Remembered: true, Authenticated: true},
	{Name: "Stranger", Address: 0xAABBCCDDEE04, Class: 0x240404, Connected: true}, // connected, not paired
}

func newTestCLI(adapter bt.Adapter) (c *cli, stdout, stderr *bytes.Buffer) {
	stdout, stderr = new(bytes.Buffer), new(bytes.Buffer)
	c = &cli{
		stdout:     stdout,
		stderr:     stderr,
		newAdapter: func(*slog.Logger) (bt.Adapter, error) { return adapter, nil },
		openLog:    func(bool) (*slog.Logger, func()) { return slog.New(slog.DiscardHandler), func() {} },
	}
	return c, stdout, stderr
}

func TestRunWithoutArguments(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{})
	if code := c.run(nil); code != 2 || !strings.Contains(stderr.String(), "Usage:") {
		t.Errorf("run() = %d, stderr %q; want 2 and the usage", code, stderr)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{})
	if code := c.run([]string{"frobnicate"}); code != 2 || !strings.Contains(stderr.String(), `unknown command "frobnicate"`) {
		t.Errorf("run() = %d, stderr %q; want 2 and the unknown command", code, stderr)
	}
}

func TestRunVersionAndHelp(t *testing.T) {
	c, stdout, _ := newTestCLI(staticAdapter{})
	if code := c.run([]string{"version"}); code != 0 || !strings.HasPrefix(stdout.String(), "bluelatch dev (go") {
		t.Errorf("version: code %d, stdout %q", code, stdout)
	}
	stdout.Reset()
	if code := c.run([]string{"help"}); code != 0 || !strings.Contains(stdout.String(), "bluelatch connect") {
		t.Errorf("help: code %d, stdout %q", code, stdout)
	}
}

func TestListShowsThePairedAudioDevices(t *testing.T) {
	c, stdout, stderr := newTestCLI(staticAdapter{devices: testDevices})

	if code := c.run([]string{"list"}); code != 0 {
		t.Fatalf("list = %d, stderr %q", code, stderr)
	}
	out := stdout.String()
	for _, want := range []string{"Example Speaker", "AA:BB:CC:DD:EE:01", "Audio/Video: Loudspeaker", "Example Buds", "not connected"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
	for _, unwanted := range []string{"Example Mouse", "Stranger"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output shows %q, which is not a paired audio device:\n%s", unwanted, out)
		}
	}
}

func TestListAllShowsEveryDevice(t *testing.T) {
	c, stdout, _ := newTestCLI(staticAdapter{devices: testDevices})

	if code := c.run([]string{"list", "-all"}); code != 0 {
		t.Fatalf("list -all = %d", code)
	}
	for _, d := range testDevices {
		if !strings.Contains(stdout.String(), d.Name) {
			t.Errorf("output lacks %q:\n%s", d.Name, stdout)
		}
	}
}

func TestListVerboseShowsTheWindowsFlagsAndServices(t *testing.T) {
	c, stdout, _ := newTestCLI(staticAdapter{devices: testDevices})

	if code := c.run([]string{"list", "-v"}); code != 0 {
		t.Fatalf("list -v = %d", code)
	}
	for _, want := range []string{"DETAILS", "authenticated=true", "class=0x248404", "services: A2DP Sink, Hands-Free"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, stdout)
		}
	}
}

func TestListWithoutDevices(t *testing.T) {
	c, stdout, _ := newTestCLI(staticAdapter{})
	if code := c.run([]string{"list"}); code != 0 || !strings.Contains(stdout.String(), "No paired Bluetooth audio devices found") {
		t.Errorf("list = %d, stdout %q", code, stdout)
	}
}

func TestListReportsAnAdapterError(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{err: bt.ErrNoRadio})
	if code := c.run([]string{"list"}); code != 1 || !strings.Contains(stderr.String(), "no Bluetooth radio found") {
		t.Errorf("list = %d, stderr %q; want 1 and the error", code, stderr)
	}
}

func TestListRejectsExtraArguments(t *testing.T) {
	c, _, _ := newTestCLI(staticAdapter{})
	if code := c.run([]string{"list", "extra"}); code != 2 {
		t.Errorf("list extra = %d, want 2", code)
	}
}

func TestConnectToAConnectedDeviceReturnsAtOnce(t *testing.T) {
	c, stdout, stderr := newTestCLI(staticAdapter{devices: testDevices})

	// The flag stands after the name, and the name is not quoted.
	if code := c.run([]string{"connect", "Example", "Speaker", "-v"}); code != 0 {
		t.Fatalf("connect = %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout.String(), "Connected after 0 attempt(s)") {
		t.Errorf("stdout = %q, want the success line", stdout)
	}
}

func TestConnectSwitchesOnServicesAnEarlierRunLeftOff(t *testing.T) {
	called := false
	c, stdout, stderr := newTestCLI(recoveringAdapter{
		staticAdapter: staticAdapter{devices: testDevices},
		restored:      []bt.Restored{{Device: "Example Buds", Address: 0xAABBCCDDEE02, Service: bt.ServiceA2DPSink}},
		err:           errors.New("one record could not be read"),
		called:        &called,
	})

	if code := c.run([]string{"connect", "Example Speaker"}); code != 0 {
		t.Fatalf("connect = %d, stderr %q", code, stderr)
	}
	if !called {
		t.Error("Recover was not called before connecting")
	}
	if !strings.Contains(stdout.String(), "Switched on the A2DP Sink service of Example Buds again") {
		t.Errorf("stdout = %q, want the notice about the restored service", stdout)
	}
	if !strings.Contains(stderr.String(), "one record could not be read") {
		t.Errorf("stderr = %q, want the warning", stderr)
	}
}

func TestListDoesNotRecover(t *testing.T) {
	called := false
	c, _, _ := newTestCLI(recoveringAdapter{staticAdapter: staticAdapter{devices: testDevices}, called: &called})
	if code := c.run([]string{"list"}); code != 0 || called {
		t.Errorf("list = %d, Recover called = %v; want a read-only command that recovers nothing", code, called)
	}
}

func TestConnectRefusesWhatItCannotFind(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{devices: testDevices})
	if code := c.run([]string{"connect", "sony"}); code != 1 || !strings.Contains(stderr.String(), `no device matches "sony"`) {
		t.Errorf("connect = %d, stderr %q", code, stderr)
	}
}

func TestConnectRefusesADeviceThatIsNotAudio(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{devices: testDevices})
	if code := c.run([]string{"connect", "Example Mouse"}); code != 1 || !strings.Contains(stderr.String(), "is paired but is not an audio device") {
		t.Errorf("connect = %d, stderr %q", code, stderr)
	}
}

func TestConnectNeedsAName(t *testing.T) {
	c, _, stderr := newTestCLI(staticAdapter{devices: testDevices})
	if code := c.run([]string{"connect"}); code != 2 || !strings.Contains(stderr.String(), "give the name or the address") {
		t.Errorf("connect = %d, stderr %q", code, stderr)
	}
}

func TestPrintProgress(t *testing.T) {
	tests := []struct {
		p    latch.Progress
		want string
	}{
		{latch.Progress{Phase: latch.PhaseReconnecting, Attempt: 1}, "[00:00.0] attempt 1: asking Windows to connect the device ...\n"},
		{latch.Progress{Phase: latch.PhaseWaiting, Attempt: 1, Elapsed: time.Second, Detail: "A2DP Sink: ok"},
			"[00:01.0] attempt 1: Windows answered (A2DP Sink: ok); waiting for the link\n"},
		{latch.Progress{Phase: latch.PhaseWaiting, Attempt: 2, Elapsed: 14 * time.Second, Detail: "x", Err: errors.New("boom")},
			"[00:14.0] attempt 2: Windows answered (boom); waiting for the link\n"},
		{latch.Progress{Phase: latch.PhaseBackoff, Attempt: 2, Elapsed: 13 * time.Second, Delay: 3 * time.Second},
			"[00:13.0] attempt 2: not connected; next attempt in 3s\n"},
		{latch.Progress{Phase: latch.PhaseConnected, Elapsed: 31500 * time.Millisecond}, "[00:31.5] the device is connected\n"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		printProgress(&out, tt.p)
		if out.String() != tt.want {
			t.Errorf("printProgress(%v) = %q, want %q", tt.p.Phase, out.String(), tt.want)
		}
	}
}

func TestElapsedText(t *testing.T) {
	for d, want := range map[time.Duration]string{
		0:                        "00:00.0",
		65300 * time.Millisecond: "01:05.3",
		59960 * time.Millisecond: "01:00.0",
		12 * time.Minute:         "12:00.0",
	} {
		if got := elapsedText(d); got != want {
			t.Errorf("elapsedText(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestParseArgsAcceptsFlagsAnywhere(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	verbose := fs.Bool("v", false, "")
	words, err := parseArgs(fs, []string{"Example", "-v", "Buds", "Two"})
	if err != nil || !*verbose || strings.Join(words, " ") != "Example Buds Two" {
		t.Errorf("parseArgs = %v, %v, verbose %v; want the three words and the flag", words, err, *verbose)
	}
	if _, err := parseArgs(fs, []string{"-nope"}); err == nil {
		t.Error("parseArgs accepted an unknown flag")
	}
}
