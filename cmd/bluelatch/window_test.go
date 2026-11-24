package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

// fakeConsole records what the program asks of the console.
type fakeConsole struct {
	state    consoleState
	freed    int
	started  [][]string
	startErr error
}

func (f *fakeConsole) State() consoleState { return f.state }
func (f *fakeConsole) Free()               { f.freed++ }
func (f *fakeConsole) StartDetached(args []string) error {
	f.started = append(f.started, args)
	return f.startErr
}

// windowRig is a cli whose console, message box, and window are fakes.
type windowRig struct {
	c              *cli
	console        *fakeConsole
	stdout, stderr *bytes.Buffer
	opened         []windowOptions // the windows opened in this process
	messages       []string        // the message boxes shown
	exit           int             // what the window returns when it is closed
}

func newWindowRig(state consoleState) *windowRig {
	r := &windowRig{console: &fakeConsole{state: state}}
	c, stdout, stderr := newTestCLI(staticAdapter{})
	c.console = r.console
	c.showError = func(title, text string) { r.messages = append(r.messages, title+": "+text) }
	c.window = func(o windowOptions) int {
		r.opened = append(r.opened, o)
		return r.exit
	}
	r.c, r.stdout, r.stderr = c, stdout, stderr
	return r
}

func TestWindowFlags(t *testing.T) {
	tests := []struct {
		args []string
		want windowOptions
	}{
		{nil, windowOptions{lang: "auto"}},
		{[]string{"-lang", "zh", "-v", "-debug"}, windowOptions{lang: "zh", verbose: true, debug: true}},
		{[]string{"-demo", "-demo-connect", "Demo Earbuds"}, windowOptions{lang: "auto", demo: true, demoConnect: "Demo Earbuds"}},
		{[]string{"-demo", "-demo-disconnect", "Demo Speaker"}, windowOptions{lang: "auto", demo: true, demoDisconnect: "Demo Speaker"}},
	}
	for _, tt := range tests {
		got, err := parseWindowFlags(tt.args)
		if err != nil || got != tt.want {
			t.Errorf("parseWindowFlags(%q) = %+v, %v; want %+v", tt.args, got, err, tt.want)
		}
	}
}

func TestWindowFlagsAreChecked(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-lang", "fr"}, `-lang "fr" is not auto, en or zh`},
		{[]string{"-demo-connect", "X"}, "need -demo"},
		{[]string{"-demo-disconnect", "X"}, "need -demo"},
		{[]string{"-demo", "-demo-connect", "X", "-demo-disconnect", "Y"}, "not both"},
		{[]string{"-nope"}, "flag provided but not defined: -nope"},
		{[]string{"extra"}, `unexpected argument "extra"`},
	}
	for _, tt := range tests {
		if _, err := parseWindowFlags(tt.args); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("parseWindowFlags(%q) error = %v, want one that contains %q", tt.args, err, tt.want)
		}
	}
	if _, err := parseWindowFlags([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Errorf("parseWindowFlags(-h) error = %v, want flag.ErrHelp", err)
	}
}

func TestRunWithoutArgumentsOpensTheWindow(t *testing.T) {
	r := newWindowRig(consoleOwn)
	if code := r.c.run(nil); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	if len(r.opened) != 1 || r.opened[0] != (windowOptions{lang: "auto"}) {
		t.Errorf("windows opened = %+v, want one with the defaults", r.opened)
	}
	if r.stdout.Len() != 0 || r.stderr.Len() != 0 {
		t.Errorf("output on the console that is being closed: %q %q", r.stdout, r.stderr)
	}
}

func TestTheConsoleOfADoubleClickIsFreedBeforeTheWindowOpens(t *testing.T) {
	r := newWindowRig(consoleOwn)
	freedWhenOpened := -1
	r.c.window = func(windowOptions) int {
		freedWhenOpened = r.console.freed
		return 0
	}
	r.c.run([]string{"window"})
	if freedWhenOpened != 1 {
		t.Errorf("the console had been freed %d times when the window opened, want 1", freedWhenOpened)
	}
	if len(r.console.started) != 0 {
		t.Errorf("another process was started: %q", r.console.started)
	}
}

func TestTheExitCodeOfTheWindowIsTheExitCodeOfTheProgram(t *testing.T) {
	r := newWindowRig(consoleNone)
	r.exit = 1
	if code := r.c.run([]string{"window"}); code != 1 {
		t.Errorf("run(window) = %d, want 1", code)
	}
}

func TestWindowWithoutAConsoleOpensHere(t *testing.T) {
	r := newWindowRig(consoleNone)
	if code := r.c.run([]string{"window", "-lang", "zh", "-demo"}); code != 0 {
		t.Fatalf("run(window) = %d, want 0", code)
	}
	if len(r.opened) != 1 || r.opened[0] != (windowOptions{lang: "zh", demo: true}) {
		t.Errorf("windows opened = %+v, want one with -lang zh -demo", r.opened)
	}
	if r.console.freed != 0 || len(r.console.started) != 0 {
		t.Errorf("freed %d times, started %q; a process without a console has nothing to free or to start", r.console.freed, r.console.started)
	}
}

func TestWindowFromATerminalOpensInAnotherProcess(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want []string
	}{
		{nil, []string{"window"}},
		{[]string{"window"}, []string{"window"}},
		{[]string{"window", "-lang", "zh", "-demo"}, []string{"window", "-lang", "zh", "-demo"}},
	} {
		r := newWindowRig(consoleShared)
		if code := r.c.run(tt.args); code != 0 {
			t.Errorf("run(%q) = %d, want 0", tt.args, code)
		}
		if len(r.console.started) != 1 || strings.Join(r.console.started[0], " ") != strings.Join(tt.want, " ") {
			t.Errorf("run(%q) started %q, want one process with %q", tt.args, r.console.started, tt.want)
		}
		if len(r.opened) != 0 || r.console.freed != 0 {
			t.Errorf("run(%q): windows opened here %d, console freed %d times; the terminal must stay as it is", tt.args, len(r.opened), r.console.freed)
		}
	}
}

func TestWindowFromATerminalReportsAStartFailure(t *testing.T) {
	r := newWindowRig(consoleShared)
	r.console.startErr = errors.New("no such file")
	if code := r.c.run(nil); code != 1 || !strings.Contains(r.stderr.String(), "could not be started: no such file") {
		t.Errorf("run() = %d, stderr %q; want 1 and the reason", code, r.stderr)
	}
}

func TestWindowFromATerminalReportsAMistakeOnTheTerminal(t *testing.T) {
	r := newWindowRig(consoleShared)
	if code := r.c.run([]string{"window", "-lang", "fr"}); code != 2 {
		t.Errorf("run(window -lang fr) = %d, want 2", code)
	}
	for _, want := range []string{`-lang "fr" is not auto, en or zh`, "bluelatch window [-lang"} {
		if !strings.Contains(r.stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, r.stderr)
		}
	}
	if len(r.messages) != 0 || len(r.console.started) != 0 || len(r.opened) != 0 {
		t.Errorf("message boxes %q, processes started %q, windows opened %d; a mistake must stop everything", r.messages, r.console.started, len(r.opened))
	}
}

func TestWindowHelpFromATerminal(t *testing.T) {
	r := newWindowRig(consoleShared)
	if code := r.c.run([]string{"window", "-h"}); code != 0 || !strings.Contains(r.stdout.String(), "-demo-disconnect NAME") {
		t.Errorf("run(window -h) = %d, stdout %q; want 0 and the usage", code, r.stdout)
	}
	if len(r.console.started) != 0 || len(r.opened) != 0 {
		t.Error("help opened a window")
	}
}

func TestWindowMistakesAreShownInAMessageBoxWhenNoTerminalIsThere(t *testing.T) {
	for _, state := range []consoleState{consoleNone, consoleOwn} {
		r := newWindowRig(state)
		if code := r.c.run([]string{"window", "-lang", "fr"}); code != 2 {
			t.Errorf("state %d: run(window -lang fr) = %d, want 2", state, code)
		}
		if len(r.messages) != 1 || !strings.Contains(r.messages[0], `-lang "fr" is not auto, en or zh`) || !strings.Contains(r.messages[0], "bluelatch window [-lang") {
			t.Errorf("state %d: message boxes %q, want one with the mistake and the usage", state, r.messages)
		}
		if r.stderr.Len() != 0 || len(r.opened) != 0 {
			t.Errorf("state %d: stderr %q, windows opened %d", state, r.stderr, len(r.opened))
		}
	}
}

func TestHelpMentionsTheWindow(t *testing.T) {
	r := newWindowRig(consoleShared)
	if code := r.c.run([]string{"help"}); code != 0 {
		t.Fatalf("help = %d, want 0", code)
	}
	for _, want := range []string{"bluelatch window", "Without a command, BlueLatch opens its window"} {
		if !strings.Contains(r.stdout.String(), want) {
			t.Errorf("help lacks %q:\n%s", want, r.stdout)
		}
	}
	if len(r.console.started) != 0 || len(r.opened) != 0 {
		t.Error("help opened a window")
	}
}
