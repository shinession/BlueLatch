package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

// windowUsage describes the flags of bluelatch window.
const windowUsage = `bluelatch window [-lang auto|en|zh] [-v] [-debug] [-demo [-demo-connect NAME] [-demo-disconnect NAME]]

  -lang    language of the window: auto follows Windows (default), en, or zh
  -v       log every read of the device state to the log file
  -debug   switch on the developer tools and the context menu of the window
  -demo    use made-up devices, and touch no real device
  -demo-connect NAME
           with -demo, start connecting that device a moment after the window is up
  -demo-disconnect NAME
           with -demo, start disconnecting that device a moment after the window is up`

// windowOptions are the flags of bluelatch window.
type windowOptions struct {
	lang           string // auto, en or zh
	verbose        bool   // log every read of the device state to the log file
	debug          bool   // switch on the developer tools and the context menu
	demo           bool   // use made-up devices
	demoConnect    string // with demo: start connecting this device a moment after the window is up
	demoDisconnect string // with demo: start disconnecting this device a moment after the window is up
}

// parseWindowFlags reads the flags of bluelatch window. The error says what is
// wrong; it is flag.ErrHelp for -h.
func parseWindowFlags(args []string) (windowOptions, error) {
	fs := flag.NewFlagSet("window", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // the caller decides where a mistake is shown
	var o windowOptions
	fs.StringVar(&o.lang, "lang", "auto", "")
	fs.BoolVar(&o.verbose, "v", false, "")
	fs.BoolVar(&o.debug, "debug", false, "")
	fs.BoolVar(&o.demo, "demo", false, "")
	fs.StringVar(&o.demoConnect, "demo-connect", "", "")
	fs.StringVar(&o.demoDisconnect, "demo-disconnect", "", "")
	if err := fs.Parse(args); err != nil {
		return windowOptions{}, err
	}
	switch {
	case fs.NArg() > 0:
		return windowOptions{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	case o.lang != "auto" && o.lang != "en" && o.lang != "zh":
		return windowOptions{}, fmt.Errorf("-lang %q is not auto, en or zh", o.lang)
	case (o.demoConnect != "" || o.demoDisconnect != "") && !o.demo:
		return windowOptions{}, errors.New("-demo-connect and -demo-disconnect need -demo")
	case o.demoConnect != "" && o.demoDisconnect != "":
		return windowOptions{}, errors.New("use either -demo-connect or -demo-disconnect, not both")
	}
	return o, nil
}

// consoleState says how this process is attached to a console.
type consoleState int

const (
	// consoleNone: there is no console. The process was started without one, for
	// instance by another bluelatch that opens the window for a terminal.
	consoleNone consoleState = iota
	// consoleOwn: a console that only this process uses. Windows made it when the
	// program was started by a double click, from a shortcut, or from the Start
	// menu, because the program is a console program.
	consoleOwn
	// consoleShared: the console of a terminal, which other processes use too.
	consoleShared
)

// console is what the program needs from the console of Windows.
type console interface {
	// State says how this process is attached to a console.
	State() consoleState
	// Free detaches this process from its console. A console that no other process
	// uses is closed, window included.
	Free()
	// StartDetached starts this program again with the arguments, without a console,
	// and returns without waiting for it.
	StartDetached(args []string) error
}

// openWindow runs bluelatch and bluelatch window. What it does depends on how
// the program was started (ADR 0008):
//
//   - Alone on a console of its own (a double click, a shortcut): Windows made a
//     console window for it. The window is closed before anything else, so that
//     it is seen as briefly as possible, and the window of BlueLatch opens in
//     this process.
//   - From a terminal, whose console other processes use too: the window opens
//     in another process, so that the prompt comes back at once instead of
//     waiting until the window is closed.
//   - Without a console: the window opens in this process.
func (c *cli) openWindow(args []string) int {
	state := c.console.State()
	if state == consoleOwn {
		c.console.Free()
	}
	o, err := parseWindowFlags(args)
	if err != nil {
		return c.windowProblem(state, err)
	}
	if state == consoleShared {
		if err := c.console.StartDetached(append([]string{"window"}, args...)); err != nil {
			fmt.Fprintln(c.stderr, "bluelatch: the window could not be started:", err)
			return 1
		}
		return 0
	}
	return c.window(o)
}

// windowProblem reports a mistake in the command line of the window: on the
// terminal when there is one to read it, otherwise in a message box. It returns
// the exit code.
func (c *cli) windowProblem(state consoleState, err error) int {
	help := errors.Is(err, flag.ErrHelp)
	code := 2
	if help {
		code = 0
	}
	if state == consoleShared {
		if help {
			fmt.Fprintln(c.stdout, windowUsage)
		} else {
			fmt.Fprintf(c.stderr, "bluelatch window: %v\n\n%s\n", err, windowUsage)
		}
		return code
	}
	text := windowUsage
	if !help {
		text = err.Error() + "\n\n" + windowUsage
	}
	c.showError("BlueLatch", text)
	return code
}
