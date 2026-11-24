package main

import (
	"flag"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
)

func (c *cli) list(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(c.stderr)
	all := fs.Bool("all", false, "also show the devices that are not paired audio devices")
	verbose := fs.Bool("v", false, "show the radio, the Windows flags, the class and the services of each device")
	words, err := parseArgs(fs, args)
	if err != nil {
		return 2
	}
	if len(words) != 0 {
		fmt.Fprintln(c.stderr, "bluelatch list: unexpected argument", words[0])
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

	if *verbose {
		c.printRadios(adapter)
	}
	var shown []bt.Device
	for _, d := range devices {
		if *all || (d.Paired() && d.IsAudio()) {
			shown = append(shown, d)
		}
	}
	if len(shown) == 0 {
		if *all {
			fmt.Fprintln(c.stdout, "Windows reports no Bluetooth devices.")
		} else {
			fmt.Fprintln(c.stdout, "No paired Bluetooth audio devices found. Pair one in Windows Settings first, or use -all to see every device.")
		}
		return 0
	}

	// The details are the last cell of a row, which does not take part in the
	// column widths, so the first four columns stay aligned.
	tw := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	if *verbose {
		fmt.Fprintln(tw, "NAME\tADDRESS\tSTATE\tTYPE\tDETAILS")
	} else {
		fmt.Fprintln(tw, "NAME\tADDRESS\tSTATE\tTYPE")
	}
	for _, d := range shown {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s", d.DisplayName(), d.Address, stateText(d), d.Class.Describe())
		if *verbose {
			fmt.Fprintf(tw, "\t%s", verboseText(d))
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
	return 0
}

func (c *cli) printRadios(adapter bt.Adapter) {
	lister, ok := adapter.(bt.RadioLister)
	if !ok {
		return
	}
	radios, err := lister.Radios()
	if err != nil {
		fmt.Fprintln(c.stderr, "bluelatch: reading the radios:", err)
		return
	}
	for _, r := range radios {
		fmt.Fprintf(c.stdout, "Radio: %s %s (manufacturer 0x%04X, LMP subversion 0x%04X)\n", r.Name, r.Address, r.Manufacturer, r.LMPSubversion)
	}
	fmt.Fprintln(c.stdout)
}

func stateText(d bt.Device) string {
	switch {
	case d.Connected:
		return "connected"
	case d.Paired():
		return "not connected"
	}
	return "not paired"
}

// verboseText describes what Windows reports for a device, on one line.
func verboseText(d bt.Device) string {
	services := make([]string, 0, len(d.Services))
	for _, s := range d.Services {
		services = append(services, s.String())
	}
	if len(services) == 0 {
		services = append(services, "none")
	}
	return fmt.Sprintf("connected=%t remembered=%t authenticated=%t class=0x%06X audio=%t last-seen=%s last-used=%s services: %s",
		d.Connected, d.Remembered, d.Authenticated, uint32(d.Class), d.IsAudio(), timeText(d.LastSeen), timeText(d.LastUsed), strings.Join(services, ", "))
}

func timeText(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}
