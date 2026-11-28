package audio

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const (
	idSpeaker = "{0.0.0.00000000}.{example-speaker}"
	idMonitor = "{0.0.0.00000000}.{example-monitor}"
	idHeadset = "{0.0.0.00000000}.{example-headset}"
)

// fakeAdapter has a speaker, a monitor and a headset, and records the changes.
type fakeAdapter struct {
	defaults  Defaults
	stateErr  error
	setErr    map[Role]error
	ignoreSet bool // SetDefault reports success but changes nothing
	sets      []string
}

func newFake(console, multimedia, communications string) *fakeAdapter {
	return &fakeAdapter{defaults: Defaults{console, multimedia, communications}}
}

func fakeOutputs() []Output {
	return []Output{
		{ID: idHeadset, Name: "Headset (Example Buds)", Kind: KindHeadset},
		{ID: idMonitor, Name: "Example Monitor (Example Display Audio)", Kind: KindDisplay},
		{ID: idSpeaker, Name: "Speakers (Example Speaker)", Kind: KindSpeaker},
	}
}

func (f *fakeAdapter) State() (State, error) {
	if f.stateErr != nil {
		return State{}, f.stateErr
	}
	return State{Outputs: fakeOutputs(), Defaults: f.defaults}, nil
}

func (f *fakeAdapter) SetDefault(id string, role Role) error {
	if err := f.setErr[role]; err != nil {
		return err
	}
	f.sets = append(f.sets, role.String())
	if !f.ignoreSet {
		f.defaults[role] = id
	}
	return nil
}

func TestSwitchMovesTheCallsAlongWhenTheySharedTheOutput(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idSpeaker)

	res, err := Switch(a, idMonitor, nil)

	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if a.defaults != (Defaults{idMonitor, idMonitor, idMonitor}) {
		t.Errorf("defaults = %v, want the monitor for all three roles", a.defaults)
	}
	if res.From.ID != idSpeaker || res.To.ID != idMonitor || res.Calls.ID != idMonitor {
		t.Errorf("result = %+v, want from the speaker to the monitor, calls included", res)
	}
	if !slices.Equal(res.Changed, []Role{RoleConsole, RoleMultimedia, RoleCommunications}) {
		t.Errorf("changed = %v, want all three roles", res.Changed)
	}
}

func TestSwitchLeavesASeparateCallsOutputAlone(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idHeadset)

	res, err := Switch(a, idMonitor, nil)

	if err != nil {
		t.Fatalf("Switch: %v", err)
	}
	if a.defaults != (Defaults{idMonitor, idMonitor, idHeadset}) {
		t.Errorf("defaults = %v, want the calls to stay on the headset", a.defaults)
	}
	if res.Calls.ID != idHeadset {
		t.Errorf("calls = %+v, want the headset", res.Calls)
	}
	if !slices.Equal(a.sets, []string{"console", "multimedia"}) {
		t.Errorf("SetDefault calls = %v, want console and multimedia only", a.sets)
	}
}

// The rule looks at what is set now and remembers nothing: calls that were set to
// the output that is switched to are with the default from then on.
func TestCallsFollowOnceTheyShareTheDefault(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idHeadset)

	for _, id := range []string{idHeadset, idSpeaker} {
		if _, err := Switch(a, id, nil); err != nil {
			t.Fatalf("Switch(%s): %v", id, err)
		}
	}

	if a.defaults != (Defaults{idSpeaker, idSpeaker, idSpeaker}) {
		t.Errorf("defaults = %v, want the speaker for all three roles", a.defaults)
	}
}

func TestSwitchToTheCurrentOutputChangesNothing(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idHeadset)

	res, err := Switch(a, idSpeaker, nil)

	if err != nil || len(res.Changed) != 0 || len(a.sets) != 0 {
		t.Errorf("Switch = %+v, %v, SetDefault %v; want nothing to be changed", res, err, a.sets)
	}
}

func TestSwitchChangesOnlyTheRolesThatDiffer(t *testing.T) {
	a := newFake(idSpeaker, idMonitor, idSpeaker)

	if _, err := Switch(a, idMonitor, nil); err != nil {
		t.Fatalf("Switch: %v", err)
	}

	if !slices.Equal(a.sets, []string{"console", "communications"}) {
		t.Errorf("SetDefault calls = %v, want the two roles that did not have the monitor", a.sets)
	}
}

func TestSwitchWithoutADefaultSetsEveryRole(t *testing.T) {
	a := newFake("", "", "")

	res, err := Switch(a, idMonitor, nil)

	if err != nil || len(res.Changed) != 3 || res.From.ID != "" {
		t.Errorf("Switch = %+v, %v; want all three roles and no origin", res, err)
	}
}

func TestSwitchRefusesAnOutputThatIsNotThere(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idSpeaker)

	_, err := Switch(a, "{0.0.0.00000000}.{unplugged}", nil)

	if !errors.Is(err, ErrNotFound) || len(a.sets) != 0 {
		t.Errorf("Switch = %v, SetDefault %v; want ErrNotFound and no change", err, a.sets)
	}
}

func TestSwitchReportsAFailedChange(t *testing.T) {
	boom := errors.New("access denied")
	a := newFake(idSpeaker, idSpeaker, idSpeaker)
	a.setErr = map[Role]error{RoleMultimedia: boom}

	res, err := Switch(a, idMonitor, nil)

	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "multimedia") {
		t.Errorf("Switch error = %v, want it to wrap the cause and name the role", err)
	}
	if !slices.Equal(res.Changed, []Role{RoleConsole}) {
		t.Errorf("changed = %v, want what was done before the failure", res.Changed)
	}
}

func TestSwitchNoticesThatWindowsDidNotChangeTheDefault(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idSpeaker)
	a.ignoreSet = true

	if _, err := Switch(a, idMonitor, nil); !errors.Is(err, ErrNotSwitched) {
		t.Errorf("Switch = %v, want ErrNotSwitched", err)
	}
}

func TestSwitchReportsThatTheOutputsCannotBeRead(t *testing.T) {
	a := newFake(idSpeaker, idSpeaker, idSpeaker)
	a.stateErr = errors.New("boom")

	if _, err := Switch(a, idMonitor, nil); err == nil {
		t.Error("Switch succeeded although the outputs cannot be read")
	}
}

func TestNext(t *testing.T) {
	outputs := fakeOutputs()
	tests := []struct {
		current string
		want    string // the ID; empty: there is no other output
		outputs []Output
	}{
		{idHeadset, idMonitor, outputs},
		{idSpeaker, idHeadset, outputs}, // the last one wraps to the first
		{"", idHeadset, outputs},
		{"gone", idHeadset, outputs},
		{idSpeaker, "", outputs[2:]},
		{"gone", idSpeaker, outputs[2:]},
		{"", "", nil},
	}
	for _, tt := range tests {
		got, ok := Next(tt.outputs, tt.current)
		if got.ID != tt.want || ok != (tt.want != "") {
			t.Errorf("Next(%d outputs, %q) = %q, %v; want %q", len(tt.outputs), tt.current, got.ID, ok, tt.want)
		}
	}
}

func TestFind(t *testing.T) {
	outputs := fakeOutputs()
	for query, want := range map[string]string{
		"speakers (example speaker)": idSpeaker,
		"  Monitor ":                 idMonitor,
		"buds":                       idHeadset,
		strings.ToUpper(idMonitor):   idMonitor,
	} {
		got, err := Find(outputs, query)
		if err != nil || got.ID != want {
			t.Errorf("Find(%q) = %q, %v; want %q", query, got.ID, err, want)
		}
	}

	for query, want := range map[string]string{
		"example": "matches several sound outputs",
		"sony":    `no sound output matches "sony" (available: Headset (Example Buds), `,
		"  ":      "is empty",
	} {
		if _, err := Find(outputs, query); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Find(%q) = %v, want an error with %q", query, err, want)
		}
	}
	if _, err := Find(nil, "x"); err == nil || !strings.Contains(err.Error(), "there are no sound outputs") {
		t.Errorf("Find in no outputs = %v", err)
	}
}

// A whole name wins over a part of another name.
func TestFindPrefersAWholeName(t *testing.T) {
	outputs := []Output{{ID: "a", Name: "Speakers"}, {ID: "b", Name: "Speakers (Example)"}}

	got, err := Find(outputs, "speakers")

	if err != nil || got.ID != "a" {
		t.Errorf("Find = %+v, %v; want the whole name", got, err)
	}
}

func TestSortOutputs(t *testing.T) {
	outputs := []Output{{ID: "2", Name: "b"}, {ID: "3", Name: "A"}, {ID: "1", Name: "b"}}

	SortOutputs(outputs)

	var got []string
	for _, o := range outputs {
		got = append(got, o.ID)
	}
	if !slices.Equal(got, []string{"3", "1", "2"}) {
		t.Errorf("order = %v, want by name without case and then by ID", got)
	}
}
