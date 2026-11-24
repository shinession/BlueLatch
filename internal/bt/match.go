package bt

import (
	"errors"
	"fmt"
	"strings"
)

// Find returns the device the user means by query. The query is an address in
// any form ParseAddress accepts, or a name. A name matches without regard to
// case, first as a whole name and then as a part of a name, and the match must
// be unique.
func Find(devices []Device, query string) (Device, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return Device{}, errors.New("the device name or address is empty")
	}

	if addr, err := ParseAddress(query); err == nil {
		for _, d := range devices {
			if d.Address == addr {
				return d, nil
			}
		}
		return Device{}, fmt.Errorf("no device has the address %s (%s)", addr, describeAll(devices))
	}

	lower := strings.ToLower(query)
	var whole, part []Device
	for _, d := range devices {
		name := strings.ToLower(d.Name)
		switch {
		case name == lower:
			whole = append(whole, d)
		case strings.Contains(name, lower):
			part = append(part, d)
		}
	}
	for _, matches := range [][]Device{whole, part} {
		switch len(matches) {
		case 0:
		case 1:
			return matches[0], nil
		default:
			return Device{}, fmt.Errorf("%q matches several devices (%s); use the address instead", query, describeAll(matches))
		}
	}
	return Device{}, fmt.Errorf("no device matches %q (%s)", query, describeAll(devices))
}

func describeAll(devices []Device) string {
	if len(devices) == 0 {
		return "there are no devices"
	}
	parts := make([]string, 0, len(devices))
	for _, d := range devices {
		parts = append(parts, d.DisplayName()+" "+d.Address.String())
	}
	return "known: " + strings.Join(parts, ", ")
}
