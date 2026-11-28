package main

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/shinession/bluelatch/internal/audio"
)

// fakeAudio has fixed outputs and records the changes.
type fakeAudio struct {
	outputs  []audio.Output
	defaults audio.Defaults
	stateErr error
	setErr   error
	sets     int
}

func (f *fakeAudio) State() (audio.State, error) {
	return audio.State{Outputs: f.outputs, Defaults: f.defaults}, f.stateErr
}

func (f *fakeAudio) SetDefault(id string, role audio.Role) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.sets++
	f.defaults[role] = id
	return nil
}

// newFakeAudio has a monitor and a speaker, which is the default for everything.
func newFakeAudio() *fakeAudio {
	return &fakeAudio{
		outputs: []audio.Output{
			{ID: "{monitor}", Name: "Example Monitor (Example Display Audio)", Kind: audio.KindDisplay},
			{ID: "{speaker}", Name: "Speakers (Example Speaker)", Kind: audio.KindSpeaker},
		},
		defaults: audio.Defaults{"{speaker}", "{speaker}", "{speaker}"},
	}
}

func newSwitchCLI(adapter audio.Adapter) (c *cli, stdout, stderr *bytes.Buffer) {
	c, stdout, stderr = newTestCLI(staticAdapter{})
	c.newAudio = func(*slog.Logger) (audio.Adapter, error) { return adapter, nil }
	return c, stdout, stderr
}

func TestSwitchGoesToTheNextOutput(t *testing.T) {
	a := newFakeAudio()
	c, stdout, stderr := newSwitchCLI(a)

	if code := c.run([]string{"switch"}); code != 0 {
		t.Fatalf("switch = %d, stderr %q", code, stderr)
	}
	want := "Switched the sound output from Speakers (Example Speaker) to Example Monitor (Example Display Audio).\n"
	if stdout.String() != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if a.defaults != (audio.Defaults{"{monitor}", "{monitor}", "{monitor}"}) {
		t.Errorf("defaults = %v, want the monitor", a.defaults)
	}

	// Again, back to the speaker.
	stdout.Reset()
	if code := c.run([]string{"switch"}); code != 0 || a.defaults[audio.RoleConsole] != "{speaker}" {
		t.Errorf("second switch = %d, defaults %v, stdout %q", code, a.defaults, stdout)
	}
}

func TestSwitchToTheNamedOutput(t *testing.T) {
	a := newFakeAudio()
	c, stdout, stderr := newSwitchCLI(a)

	// The flag stands after the name, and the name is not quoted.
	if code := c.run([]string{"switch", "example", "monitor", "-v"}); code != 0 {
		t.Fatalf("switch = %d, stderr %q", code, stderr)
	}
	if !strings.Contains(stdout.String(), "to Example Monitor") || a.defaults[audio.RoleConsole] != "{monitor}" {
		t.Errorf("stdout = %q, defaults %v", stdout, a.defaults)
	}
}

func TestSwitchByID(t *testing.T) {
	a := newFakeAudio()
	c, _, stderr := newSwitchCLI(a)

	if code := c.run([]string{"switch", "{monitor}"}); code != 0 || a.defaults[audio.RoleConsole] != "{monitor}" {
		t.Errorf("switch = %d, defaults %v, stderr %q", code, a.defaults, stderr)
	}
}

func TestSwitchSaysWhereTheCallsStay(t *testing.T) {
	a := newFakeAudio()
	a.outputs = append([]audio.Output{{ID: "{headset}", Name: "Headset (Example Buds)", Kind: audio.KindHeadset}}, a.outputs...)
	a.defaults[audio.RoleCommunications] = "{headset}"
	c, stdout, stderr := newSwitchCLI(a)

	if code := c.run([]string{"switch", "monitor"}); code != 0 {
		t.Fatalf("switch = %d, stderr %q", code, stderr)
	}

	if !strings.HasSuffix(stdout.String(), "Calls stay on Headset (Example Buds).\n") {
		t.Errorf("stdout = %q, want where the calls stay", stdout)
	}
	if a.defaults[audio.RoleCommunications] != "{headset}" {
		t.Errorf("defaults = %v, want the calls to stay on the headset", a.defaults)
	}
}

// With the calls on the output that is switched to, there is nothing to say.
func TestSwitchSaysNothingAboutCallsThatMovedAlong(t *testing.T) {
	c, stdout, _ := newSwitchCLI(newFakeAudio())

	if code := c.run([]string{"switch"}); code != 0 || strings.Contains(stdout.String(), "Calls stay") {
		t.Errorf("switch = %d, stdout %q", code, stdout)
	}
}

func TestSwitchToTheCurrentOutput(t *testing.T) {
	a := newFakeAudio()
	c, stdout, _ := newSwitchCLI(a)

	if code := c.run([]string{"switch", "Example Speaker"}); code != 0 {
		t.Fatalf("switch = %d, want 0: the output is where the user wants it", code)
	}
	if stdout.String() != "Speakers (Example Speaker) is already the sound output.\n" || a.sets != 0 {
		t.Errorf("stdout = %q after %d changes", stdout, a.sets)
	}
}

func TestSwitchWithoutAnotherOutput(t *testing.T) {
	a := newFakeAudio()
	a.outputs = a.outputs[1:] // the speaker only
	c, _, stderr := newSwitchCLI(a)

	if code := c.run([]string{"switch"}); code != 1 || !strings.Contains(stderr.String(), "no other sound output") || a.sets != 0 {
		t.Errorf("switch = %d, stderr %q after %d changes", code, stderr, a.sets)
	}
}

func TestSwitchRefusesWhatItCannotFind(t *testing.T) {
	c, _, stderr := newSwitchCLI(newFakeAudio())

	if code := c.run([]string{"switch", "sony"}); code != 1 || !strings.Contains(stderr.String(), `no sound output matches "sony" (available: `) {
		t.Errorf("switch = %d, stderr %q", code, stderr)
	}
	stderr.Reset()
	if code := c.run([]string{"switch", "example"}); code != 1 || !strings.Contains(stderr.String(), "matches several sound outputs") {
		t.Errorf("switch = %d, stderr %q", code, stderr)
	}
}

func TestSwitchReportsErrors(t *testing.T) {
	read := newFakeAudio()
	read.stateErr = errors.New("boom")
	c, _, stderr := newSwitchCLI(read)
	if code := c.run([]string{"switch"}); code != 1 || !strings.Contains(stderr.String(), "boom") {
		t.Errorf("switch = %d, stderr %q; want 1 and the error", code, stderr)
	}

	write := newFakeAudio()
	write.setErr = audio.ErrUnsupported
	c, _, stderr = newSwitchCLI(write)
	if code := c.run([]string{"switch"}); code != 1 || !strings.Contains(stderr.String(), "does not offer a way") {
		t.Errorf("switch = %d, stderr %q; want 1 and the error", code, stderr)
	}
}

func TestSwitchRejectsAnUnknownFlag(t *testing.T) {
	a := newFakeAudio()
	c, _, _ := newSwitchCLI(a)
	if code := c.run([]string{"switch", "-nope"}); code != 2 || a.sets != 0 {
		t.Errorf("switch -nope = %d after %d changes, want 2 and no change", code, a.sets)
	}
}

func TestTheUsageNamesSwitch(t *testing.T) {
	c, stdout, _ := newTestCLI(staticAdapter{})
	if code := c.run([]string{"help"}); code != 0 || !strings.Contains(stdout.String(), "bluelatch switch [-v] [<name or ID>]") {
		t.Errorf("help = %d, stdout %q", code, stdout)
	}
}
