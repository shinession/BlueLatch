// Package audio is the sound outputs of Windows: which ones can be used now,
// which one is the default, and how to change it. The types, the interface and
// the rule for switching are here; the Core Audio calls are in the
// *_windows.go files.
package audio

import (
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

var (
	// ErrNotFound means the output is not among the active ones, for example
	// because it was unplugged.
	ErrNotFound = errors.New("the sound output is not available")

	// ErrNotSwitched means Windows accepted the change, but the default is still
	// another output.
	ErrNotSwitched = errors.New("Windows did not change the default output")

	// ErrUnsupported means Windows does not offer the way BlueLatch changes the
	// default output (ADR 0009).
	ErrUnsupported = errors.New("Windows does not offer a way to change the default output")
)

// Role is one of the jobs Windows has a default output for. The values are
// those of the ERole enumeration of Windows.
type Role int

const (
	RoleConsole        Role = iota // system sounds, games
	RoleMultimedia                 // music and films
	RoleCommunications             // calls
)

// String names the role, for the log and for errors.
func (r Role) String() string {
	switch r {
	case RoleConsole:
		return "console"
	case RoleMultimedia:
		return "multimedia"
	case RoleCommunications:
		return "communications"
	}
	return fmt.Sprintf("role %d", int(r))
}

// Kind says what an output is.
type Kind string

const (
	KindSpeaker    Kind = "speaker"
	KindHeadphones Kind = "headphones"
	KindHeadset    Kind = "headset"
	KindDisplay    Kind = "display" // the speakers of a monitor or a television
	KindOther      Kind = "audio"
)

// Output is a sound output that can be used now.
type Output struct {
	ID   string // what Windows calls it
	Name string // the name Windows shows, for example "Speakers (Example Speaker)"
	Kind Kind
}

// Defaults holds the ID of the default output of every role, indexed by Role.
// An empty ID means there is none.
type Defaults [3]string

// State is the outputs that can be used now and the defaults among them.
type State struct {
	Outputs  []Output // sorted by SortOutputs
	Defaults Defaults
}

// Output returns the output with the ID.
func (s State) Output(id string) (Output, bool) {
	for _, o := range s.Outputs {
		if o.ID == id {
			return o, true
		}
	}
	return Output{}, false
}

// Adapter is what the rest of the program needs from the audio system.
type Adapter interface {
	// State reads the active outputs and the defaults.
	State() (State, error)

	// SetDefault makes the output the default one of the role.
	SetDefault(id string, role Role) error
}

// Result says what Switch did.
type Result struct {
	From    Output // the default output before; zero when there was none
	To      Output
	Changed []Role // the roles whose default was changed; none when To had them all
	Calls   Output // the default for calls afterwards; zero when there is none
}

// Switch makes the output with the ID the default output of Windows: the one of
// the console and multimedia roles, which Windows shows as selected. The default
// for calls (the communications role) follows when it is the same as the default
// output now; a separate one is left alone. A role that has the output already
// is not touched. The change is checked by reading the defaults again. log may
// be nil.
func Switch(a Adapter, id string, log *slog.Logger) (Result, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	state, err := a.State()
	if err != nil {
		return Result{}, err
	}
	to, ok := state.Output(id)
	if !ok {
		return Result{}, ErrNotFound
	}
	res := Result{To: to}
	res.From, _ = state.Output(state.Defaults[RoleConsole])

	roles := []Role{RoleConsole, RoleMultimedia}
	if state.Defaults[RoleCommunications] == state.Defaults[RoleConsole] {
		roles = append(roles, RoleCommunications)
	}
	for _, role := range roles {
		if state.Defaults[role] == to.ID {
			continue
		}
		if err := a.SetDefault(to.ID, role); err != nil {
			log.Warn("changing the default output failed", "output", to.Name, "role", role.String(), "err", err)
			return res, fmt.Errorf("making %s the default output (%s): %w", to.Name, role, err)
		}
		res.Changed = append(res.Changed, role)
	}

	if len(res.Changed) > 0 {
		log.Info("changed the default output", "from", res.From.Name, "to", to.Name, "roles", fmt.Sprint(res.Changed))
		if state, err = a.State(); err != nil {
			return res, fmt.Errorf("checking the default output: %w", err)
		}
		for _, role := range res.Changed {
			if state.Defaults[role] != to.ID {
				log.Warn("the default output did not change", "output", to.Name, "role", role.String())
				return res, ErrNotSwitched
			}
		}
	}
	res.Calls, _ = state.Output(state.Defaults[RoleCommunications])
	return res, nil
}

// Next returns the output after the one with the current ID, and the first one
// after the last. When the current one is not in the list, it returns the first.
// ok is false when there is no other output to go to.
func Next(outputs []Output, current string) (next Output, ok bool) {
	for i, o := range outputs {
		if o.ID == current {
			if len(outputs) == 1 {
				return Output{}, false
			}
			return outputs[(i+1)%len(outputs)], true
		}
	}
	if len(outputs) == 0 {
		return Output{}, false
	}
	return outputs[0], true
}

// Find returns the output the user means by query: its ID, or its name. A name
// matches without regard to case, first as a whole name and then as a part of a
// name, and the match must be unique.
func Find(outputs []Output, query string) (Output, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Output{}, errors.New("the name of the sound output is empty")
	}
	lower := strings.ToLower(query)
	var whole, part []Output
	for _, o := range outputs {
		if strings.EqualFold(o.ID, query) {
			return o, nil
		}
		name := strings.ToLower(o.Name)
		switch {
		case name == lower:
			whole = append(whole, o)
		case strings.Contains(name, lower):
			part = append(part, o)
		}
	}
	for _, matches := range [][]Output{whole, part} {
		switch len(matches) {
		case 0:
		case 1:
			return matches[0], nil
		default:
			ids := make([]string, 0, len(matches))
			for _, o := range matches {
				ids = append(ids, o.Name+" "+o.ID)
			}
			return Output{}, fmt.Errorf("%q matches several sound outputs (%s); use the ID instead", query, strings.Join(ids, ", "))
		}
	}
	if len(outputs) == 0 {
		return Output{}, fmt.Errorf("no sound output matches %q (there are no sound outputs)", query)
	}
	names := make([]string, 0, len(outputs))
	for _, o := range outputs {
		names = append(names, o.Name)
	}
	return Output{}, fmt.Errorf("no sound output matches %q (available: %s)", query, strings.Join(names, ", "))
}

// SortOutputs orders outputs by name, ignoring case, and then by ID, so the list
// does not move around when the default changes.
func SortOutputs(outputs []Output) {
	sort.SliceStable(outputs, func(i, j int) bool {
		a, b := strings.ToLower(outputs[i].Name), strings.ToLower(outputs[j].Name)
		if a != b {
			return a < b
		}
		return outputs[i].ID < outputs[j].ID
	})
}
