package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
)

func (c *cli) disconnect(args []string) int {
	policy := latch.DefaultPolicy()
	fs := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	verbose := fs.Bool("v", false, "log every state read to stderr and to the log file")
	fs.DurationVar(&policy.DisconnectTimeout, "wait", policy.DisconnectTimeout, "how long to wait for Windows to report the device as disconnected")
	words, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	query := strings.Join(words, " ")
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(c.stderr, "bluelatch disconnect: give the name or the address of a paired audio device (see bluelatch list)")
		return 2
	}

	log, closeLog := c.openLog(*verbose)
	defer closeLog()
	adapter, err := c.newAdapter(log)
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
	devices, err := adapter.Devices()
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
	dev, err := pickAudioDevice(devices, query)
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err = latch.New(adapter, policy, log).Disconnect(ctx, dev.Address)
	switch {
	case err == nil:
		fmt.Fprintf(c.stdout, "Disconnected %s (%s). It stays paired; connect it again with bluelatch connect.\n", dev.DisplayName(), dev.Address)
		return 0
	case errors.Is(err, bt.ErrNotConnected):
		fmt.Fprintf(c.stdout, "%s (%s) is not connected.\n", dev.DisplayName(), dev.Address)
		return 0
	case errors.Is(err, latch.ErrStillConnected):
		fmt.Fprintf(c.stderr, "bluelatch: %s (%s) still reports being connected %s after the request. It may have connected again by itself.\n",
			dev.DisplayName(), dev.Address, policy.DisconnectTimeout)
		return 1
	default:
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
}
