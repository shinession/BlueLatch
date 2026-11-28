package app

import (
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/shinession/bluelatch/internal/audio"
)

const (
	outSpeaker = "{example-speaker}"
	outMonitor = "{example-monitor}"
)

// fakeAudio has a speaker, which is the default, and a monitor.
type fakeAudio struct {
	mu        sync.Mutex
	defaults  audio.Defaults
	stateErr  error
	setErr    error
	ignoreSet bool
	gone      bool          // the monitor has been unplugged
	gate      chan struct{} // when set, SetDefault waits until it is closed
	sets      int
}

func newFakeAudio() *fakeAudio {
	return &fakeAudio{defaults: audio.Defaults{outSpeaker, outSpeaker, outSpeaker}}
}

func (f *fakeAudio) State() (audio.State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stateErr != nil {
		return audio.State{}, f.stateErr
	}
	outputs := []audio.Output{{ID: outSpeaker, Name: "Speakers (Example Speaker)", Kind: audio.KindSpeaker}}
	if !f.gone {
		outputs = append(outputs, audio.Output{ID: outMonitor, Name: "Example Monitor", Kind: audio.KindDisplay})
	}
	audio.SortOutputs(outputs)
	return audio.State{Outputs: outputs, Defaults: f.defaults}, nil
}

func (f *fakeAudio) SetDefault(id string, role audio.Role) error {
	f.mu.Lock()
	gate, err := f.gate, f.setErr
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	if err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets++
	if !f.ignoreSet {
		f.defaults[role] = id
	}
	return nil
}

func (f *fakeAudio) set(fn func(*fakeAudio)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

func newSoundApp(t *testing.T, sound audio.Adapter) *App {
	t.Helper()
	a := New(Config{Adapter: &fakeAdapter{}, Audio: sound, Policy: fastPolicy(), Refresh: 10 * time.Millisecond})
	a.Start()
	t.Cleanup(func() { a.Shutdown(5 * time.Second) })
	return a
}

func current(a *App) string {
	for _, o := range a.Snapshot().Sound.Outputs {
		if o.Current {
			return o.ID
		}
	}
	return ""
}

func changeDone(a *App) bool {
	c := a.Snapshot().Sound.Change
	return c != nil && c.Done
}

func TestSnapshotListsTheSoundOutputs(t *testing.T) {
	a := newSoundApp(t, newFakeAudio())

	sound := a.Snapshot().Sound

	want := []OutputView{
		{ID: outMonitor, Name: "Example Monitor", Kind: "display"},
		{ID: outSpeaker, Name: "Speakers (Example Speaker)", Kind: "speaker", Current: true},
	}
	if sound == nil || sound.Error != "" || sound.Change != nil || !slices.Equal(sound.Outputs, want) {
		t.Errorf("sound = %+v, want %+v and nothing else", sound, want)
	}
}

func TestSnapshotHasNoSoundWithoutAnAudioAdapter(t *testing.T) {
	a := newTestApp(t, &fakeAdapter{})

	if s := a.Snapshot(); s.Sound != nil {
		t.Errorf("sound = %+v, want none", s.Sound)
	}
	if err := a.SwitchOutput(outMonitor); !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("SwitchOutput = %v, want ErrUnknownOutput", err)
	}
}

func TestTheListFollowsWindows(t *testing.T) {
	f := newFakeAudio()
	a := newSoundApp(t, f)

	f.set(func(f *fakeAudio) { f.gone = true })

	waitFor(t, "the monitor to leave the list", func() bool { return len(a.Snapshot().Sound.Outputs) == 1 })
}

func TestSwitchOutputMovesTheDefault(t *testing.T) {
	f := newFakeAudio()
	f.gate = make(chan struct{})
	a := newSoundApp(t, f)

	if err := a.SwitchOutput(outMonitor); err != nil {
		t.Fatalf("SwitchOutput: %v", err)
	}
	if c := a.Snapshot().Sound.Change; c == nil || c.ID != outMonitor || c.Name != "Example Monitor" || c.Done {
		t.Errorf("change right after SwitchOutput = %+v, want a running change to the monitor", c)
	}
	if err := a.SwitchOutput(outSpeaker); !errors.Is(err, ErrSwitching) {
		t.Errorf("a second SwitchOutput = %v, want ErrSwitching", err)
	}
	close(f.gate)
	waitFor(t, "the change to end", func() bool { return changeDone(a) })

	if c := a.Snapshot().Sound.Change; c.Error != "" || c.DoneAt == 0 {
		t.Errorf("change = %+v, want success", c)
	}
	// The list is read again before the change is reported as done.
	if got := current(a); got != outMonitor {
		t.Errorf("the current output is %q, want the monitor", got)
	}
}

func TestSwitchOutputSaysWhyItFailed(t *testing.T) {
	tests := []struct {
		name  string
		fault func(*fakeAudio)
		code  string
	}{
		{"unplugged", func(f *fakeAudio) { f.gone = true }, "output-gone"},
		{"ignored", func(f *fakeAudio) { f.ignoreSet = true }, "not-switched"},
		{"unsupported", func(f *fakeAudio) { f.setErr = audio.ErrUnsupported }, "unsupported"},
		{"other", func(f *fakeAudio) { f.setErr = errors.New("access denied") }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFakeAudio()
			a := New(Config{Adapter: &fakeAdapter{}, Audio: f, Refresh: time.Hour})
			a.Start() // reads once; the next read would be in an hour
			defer a.Shutdown(5 * time.Second)
			f.set(tt.fault)

			if err := a.SwitchOutput(outMonitor); err != nil {
				t.Fatalf("SwitchOutput: %v", err)
			}
			waitFor(t, "the change to end", func() bool { return changeDone(a) })

			c := a.Snapshot().Sound.Change
			if c.Error == "" || c.ErrorCode != tt.code {
				t.Errorf("change = %+v, want an error with the code %q", c, tt.code)
			}
		})
	}
}

func TestSwitchOutputRefusesWhatIsNotInTheList(t *testing.T) {
	f := newFakeAudio()
	a := newSoundApp(t, f)

	if err := a.SwitchOutput("{nothing}"); !errors.Is(err, ErrUnknownOutput) {
		t.Errorf("SwitchOutput = %v, want ErrUnknownOutput", err)
	}
	if a.Snapshot().Sound.Change != nil {
		t.Error("a refused SwitchOutput left a change behind")
	}
}

func TestShutdownWaitsForAChangeInProgress(t *testing.T) {
	f := newFakeAudio()
	f.gate = make(chan struct{})
	a := newSoundApp(t, f)
	if err := a.SwitchOutput(outMonitor); err != nil {
		t.Fatal(err)
	}

	result := make(chan bool, 1)
	go func() { result <- a.Shutdown(5 * time.Second) }()
	select {
	case <-result:
		t.Fatal("Shutdown returned while the output was being changed")
	case <-time.After(80 * time.Millisecond):
	}
	close(f.gate)
	select {
	case ok := <-result:
		if !ok {
			t.Error("Shutdown reported a timeout although the change ended")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not return after the change ended")
	}
	if err := a.SwitchOutput(outMonitor); !errors.Is(err, ErrClosing) {
		t.Errorf("SwitchOutput after Shutdown = %v, want ErrClosing", err)
	}
}

func TestUnreadableOutputsAreReportedAndClear(t *testing.T) {
	f := newFakeAudio()
	f.stateErr = errors.New("boom")
	a := newSoundApp(t, f)

	if s := a.Snapshot().Sound; s.Error != "boom" || len(s.Outputs) != 0 {
		t.Fatalf("sound = %+v, want the error and no outputs", s)
	}
	f.set(func(f *fakeAudio) { f.stateErr = nil })
	waitFor(t, "the outputs to come back", func() bool {
		s := a.Snapshot().Sound
		return s.Error == "" && len(s.Outputs) == 2
	})
}

func TestDemoOutputsFollowTheDevices(t *testing.T) {
	d := newDemo(time.Millisecond)
	read := func() (names []string, def string) {
		state, err := d.State()
		if err != nil {
			t.Fatal(err)
		}
		for _, o := range state.Outputs {
			names = append(names, o.Name)
		}
		o, _ := state.Output(state.Defaults[audio.RoleConsole])
		return names, o.Name
	}

	names, def := read()
	want := []string{"Demo Monitor (Demo Display Audio)", "Speakers (Demo Speaker)"}
	if !slices.Equal(names, want) || def != "Speakers (Demo Speaker)" {
		t.Fatalf("outputs = %v, default %q; want %v with the speaker as the default", names, def, want)
	}

	if err := d.SetDefault(DemoMonitor, audio.RoleConsole); err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if _, def := read(); def != "Demo Monitor (Demo Display Audio)" {
		t.Errorf("default = %q, want the monitor", def)
	}
	if err := d.SetDefault("{nothing}", audio.RoleConsole); !errors.Is(err, audio.ErrNotFound) {
		t.Errorf("SetDefault(nothing) = %v, want ErrNotFound", err)
	}

	// The earbuds become an output once they are connected.
	for range 3 {
		if _, err := d.Reconnect(DemoEarbuds); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, "the earbuds to become an output", func() bool {
		names, _ := read()
		return slices.Contains(names, "Headphones (Demo Earbuds)")
	})

	// A disconnected default goes away, and the monitor takes over.
	speaker := demoOutput(d.devices[0].dev).ID
	if err := d.SetDefault(speaker, audio.RoleConsole); err != nil {
		t.Fatal(err)
	}
	if err := d.Disconnect(DemoSpeaker); err != nil {
		t.Fatal(err)
	}
	names, def = read()
	if slices.Contains(names, "Speakers (Demo Speaker)") || def != "Demo Monitor (Demo Display Audio)" {
		t.Errorf("outputs = %v, default %q; want the speaker gone and the monitor as the default", names, def)
	}
}
