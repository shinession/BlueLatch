package bt

import (
	"sort"
	"strings"
	"time"
)

// Device is a classic Bluetooth device as Windows reports it.
type Device struct {
	Name          string
	Address       Address
	Class         ClassOfDevice
	Connected     bool      // Windows reports an active connection
	Remembered    bool      // Windows keeps the device in its device list
	Authenticated bool      // pairing information exists (the device is paired)
	LastSeen      time.Time // zero when Windows has no value
	LastUsed      time.Time // zero when Windows has no value
	Services      []Service // standard services Windows has installed for the device
}

// Paired reports whether Windows holds a pairing for the device.
func (d Device) Paired() bool { return d.Authenticated || d.Remembered }

// HasService reports whether Windows has installed the service for the device.
func (d Device) HasService(s Service) bool {
	for _, have := range d.Services {
		if have == s {
			return true
		}
	}
	return false
}

// IsAudio reports whether the device is a headset, headphones, speaker or
// similar: its class is Audio/Video, or Windows installed an audio service for it.
func (d Device) IsAudio() bool {
	if d.Class.IsAudioVideo() {
		return true
	}
	for _, s := range audioServices {
		if d.HasService(s) {
			return true
		}
	}
	return false
}

// DisplayName returns the name of the device, or its address when it has none.
func (d Device) DisplayName() string {
	if strings.TrimSpace(d.Name) == "" {
		return d.Address.String()
	}
	return d.Name
}

// SortDevices orders devices by name, ignoring case, and then by address, so
// the list does not move around while the connection states change.
func SortDevices(devices []Device) {
	sort.SliceStable(devices, func(i, j int) bool {
		a, b := strings.ToLower(devices[i].Name), strings.ToLower(devices[j].Name)
		if a != b {
			return a < b
		}
		return devices[i].Address < devices[j].Address
	})
}
