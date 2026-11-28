package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/shinession/bluelatch/internal/audio"
)

// switchOutput changes the sound output of Windows: to the output the user
// named, or without a name to the one after the current one.
func (c *cli) switchOutput(args []string) int {
	fs := flag.NewFlagSet("switch", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	verbose := fs.Bool("v", false, "log what is done to stderr and to the log file")
	words, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	query := strings.TrimSpace(strings.Join(words, " "))

	log, closeLog := c.openLog(*verbose)
	defer closeLog()
	adapter, err := c.newAudio(log)
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
	state, err := adapter.State()
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}

	var target audio.Output
	if query == "" {
		next, ok := audio.Next(state.Outputs, state.Defaults[audio.RoleConsole])
		if !ok {
			fmt.Fprintln(c.stderr, "bluelatch switch: there is no other sound output to switch to")
			return 1
		}
		target = next
	} else if target, err = audio.Find(state.Outputs, query); err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}

	res, err := audio.Switch(adapter, target.ID, log)
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
	switch {
	case len(res.Changed) == 0:
		fmt.Fprintf(c.stdout, "%s is already the sound output.\n", res.To.Name)
		return 0
	case res.From.ID == "" || res.From.ID == res.To.ID:
		fmt.Fprintf(c.stdout, "Switched the sound output to %s.\n", res.To.Name)
	default:
		fmt.Fprintf(c.stdout, "Switched the sound output from %s to %s.\n", res.From.Name, res.To.Name)
	}
	if res.Calls.ID != "" && res.Calls.ID != res.To.ID {
		fmt.Fprintf(c.stdout, "Calls stay on %s.\n", res.Calls.Name)
	}
	return 0
}
