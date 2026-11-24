package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
	"github.com/shinession/bluelatch/internal/latch"
	"github.com/shinession/bluelatch/internal/logging"
)

func (c *cli) connect(args []string) int {
	policy := latch.DefaultPolicy()
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	verbose := fs.Bool("v", false, "log every state read to stderr and to the log file")
	fs.DurationVar(&policy.AttemptTimeout, "attempt-timeout", policy.AttemptTimeout, "how long one attempt waits for the link after Windows has answered")
	fs.DurationVar(&policy.StableFor, "stable", policy.StableFor, "how long the device must stay connected to count as connected")
	fs.IntVar(&policy.MaxAttempts, "max-attempts", policy.MaxAttempts, "give up after this many attempts (0 = keep trying until stopped)")
	words, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	query := strings.Join(words, " ")
	if strings.TrimSpace(query) == "" {
		fmt.Fprintln(c.stderr, "bluelatch connect: give the name or the address of a paired audio device (see bluelatch list)")
		return 2
	}

	log, closeLog := c.openLog(*verbose)
	defer closeLog()
	adapter, err := c.newAdapter(log)
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
	c.recoverServices(adapter)
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

	if path, err := logging.Path(); err == nil {
		fmt.Fprintf(c.stdout, "Log file: %s\n", path)
	}
	fmt.Fprintf(c.stdout, "Connecting %s (%s). Press Ctrl+C to stop.\n", dev.DisplayName(), dev.Address)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	engine := latch.New(adapter, policy, log)
	res, err := engine.Connect(ctx, dev.Address, func(p latch.Progress) { printProgress(c.stdout, p) })
	switch {
	case err == nil:
		fmt.Fprintf(c.stdout, "Connected after %d attempt(s) in %s.\n", res.Attempts, res.Elapsed.Round(100*time.Millisecond))
		return 0
	case errors.Is(err, context.Canceled):
		fmt.Fprintf(c.stdout, "Stopped after %d attempt(s).\n", res.Attempts)
		return 1
	default:
		fmt.Fprintln(c.stderr, "bluelatch:", err)
		return 1
	}
}

// recoverServices switches on again the services that an earlier run left
// switched off, before anything is connected.
func (c *cli) recoverServices(adapter bt.Adapter) {
	recoverer, ok := adapter.(bt.Recoverer)
	if !ok {
		return
	}
	restored, err := recoverer.Recover()
	for _, r := range restored {
		fmt.Fprintf(c.stdout, "Switched on the %s service of %s again; an earlier run had left it switched off.\n", r.Service, r.Device)
	}
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch: warning: switching on the services an earlier run left off:", err)
	}
}

// pickAudioDevice finds the paired audio device the user named. When the name
// belongs to a paired device that is not an audio device it says so, because
// BlueLatch cannot reconnect those.
func pickAudioDevice(devices []bt.Device, query string) (bt.Device, error) {
	var audio, paired []bt.Device
	for _, d := range devices {
		if !d.Paired() {
			continue
		}
		paired = append(paired, d)
		if d.IsAudio() {
			audio = append(audio, d)
		}
	}
	dev, err := bt.Find(audio, query)
	if err == nil {
		return dev, nil
	}
	if other, otherErr := bt.Find(paired, query); otherErr == nil {
		return bt.Device{}, fmt.Errorf("%s (%s) is paired but is not an audio device; BlueLatch handles classic Bluetooth audio devices only", other.DisplayName(), other.Address)
	}
	return bt.Device{}, err
}

// printProgress writes one line for a progress report.
func printProgress(w io.Writer, p latch.Progress) {
	at := "[" + elapsedText(p.Elapsed) + "]"
	outcome := p.Detail
	if p.Err != nil {
		outcome = p.Err.Error()
	}
	switch p.Phase {
	case latch.PhaseReconnecting:
		fmt.Fprintf(w, "%s attempt %d: asking Windows to connect the device ...\n", at, p.Attempt)
	case latch.PhaseWaiting:
		fmt.Fprintf(w, "%s attempt %d: Windows answered (%s); waiting for the link\n", at, p.Attempt, outcome)
	case latch.PhaseBackoff:
		fmt.Fprintf(w, "%s attempt %d: not connected; next attempt in %s\n", at, p.Attempt, p.Delay.Round(100*time.Millisecond))
	case latch.PhaseConnected:
		fmt.Fprintf(w, "%s the device is connected\n", at)
	}
}

// elapsedText formats a duration as mm:ss.t.
func elapsedText(d time.Duration) string {
	d = d.Round(100 * time.Millisecond)
	minutes := int(d / time.Minute)
	seconds := float64(d%time.Minute) / float64(time.Second)
	return fmt.Sprintf("%02d:%04.1f", minutes, seconds)
}

// parseArgs parses flags that may stand before, between or after the other
// words, which Go's flag package does not do by itself, and returns the words.
// So bluelatch connect Example Buds -v works as well as bluelatch connect -v ....
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var words []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return words, nil
		}
		words = append(words, fs.Arg(0))
		args = fs.Args()[1:]
	}
}
