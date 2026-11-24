// Command bluelatch lists the paired Bluetooth audio devices, connects one of
// them and retries until the connection holds, and disconnects one. Without a
// command it opens the window that does the same with buttons.
//
// It is one console program for both (ADR 0008): the command line needs a
// console subsystem program so that a shell waits for it and sees its exit code,
// and the window closes the console that a double click gives it.
package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/logging"
)

// version is set at build time (-ldflags "-X main.version=...").
var version = "dev"

const usage = `BlueLatch connects a paired Bluetooth audio device and keeps retrying until
the connection holds.

Usage:
  bluelatch
  bluelatch window [-lang auto|en|zh] [-v] [-debug] [-demo]
  bluelatch list [-all] [-v]
  bluelatch connect [-v] [-attempt-timeout 10s] [-stable 2s] [-max-attempts N] <name or address>
  bluelatch disconnect [-v] [-wait 5s] <name or address>
  bluelatch version

Without a command, BlueLatch opens its window, as "bluelatch window" does.
Run "bluelatch <command> -h" for the flags of a command.
`

// cli holds what the commands need from the outside, so tests can replace it.
type cli struct {
	stdout, stderr io.Writer
	newAdapter     func(*slog.Logger) (bt.Adapter, error)
	openLog        func(verbose bool) (*slog.Logger, func())

	// For the window (window.go, window_windows.go).
	console   console                  // how the program is attached to a console
	showError func(title, text string) // tells the user a problem when no terminal is there to read it
	window    func(windowOptions) int  // opens the window in this process, and returns when it is closed
}

func main() {
	c := &cli{
		stdout:     os.Stdout,
		stderr:     os.Stderr,
		newAdapter: bt.New,
	}
	c.useWindowsHost()
	c.openLog = func(verbose bool) (*slog.Logger, func()) {
		log, closeLog, err := logging.Open(verbose, c.stderr)
		if err != nil {
			fmt.Fprintf(c.stderr, "warning: the log file is not available: %v\n", err)
		}
		return log, closeLog
	}
	os.Exit(c.run(os.Args[1:]))
}

// run executes one command line and returns the exit code: 0 on success, 1 when
// the command failed and 2 for a command line that cannot be understood.
func (c *cli) run(args []string) int {
	if len(args) == 0 {
		return c.openWindow(nil)
	}
	switch args[0] {
	case "window":
		return c.openWindow(args[1:])
	case "list":
		return c.list(args[1:])
	case "connect":
		return c.connect(args[1:])
	case "disconnect":
		return c.disconnect(args[1:])
	case "version", "-version", "--version":
		fmt.Fprintf(c.stdout, "bluelatch %s (%s %s/%s)\n", version, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return 0
	case "help", "-h", "-help", "--help":
		fmt.Fprint(c.stdout, usage)
		return 0
	}
	fmt.Fprintf(c.stderr, "bluelatch: unknown command %q\n\n%s", args[0], usage)
	return 2
}
