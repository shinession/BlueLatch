package bt

import (
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrNoRadio means there is no Bluetooth radio to talk to. Windows is
	// expected to report the same while Bluetooth is turned off.
	ErrNoRadio = errors.New("no Bluetooth radio found (is Bluetooth turned on?)")

	// ErrDeviceNotFound means Windows holds no pairing for the device.
	ErrDeviceNotFound = errors.New("the device is not paired")

	// ErrNoAudioServices means no audio service is installed for the device, so
	// there is nothing to reconnect.
	ErrNoAudioServices = errors.New("the device has no installed audio service")

	// ErrNotConnected means the device is not connected, so there is nothing to
	// disconnect.
	ErrNotConnected = errors.New("the device is not connected")
)

// ServiceResult is the outcome of switching one service off and on again.
type ServiceResult struct {
	Service    Service
	DisableErr error
	EnableErr  error
}

// String describes the outcome in one short phrase.
func (r ServiceResult) String() string {
	switch {
	case r.EnableErr != nil:
		return fmt.Sprintf("%s: enable failed: %v", r.Service, r.EnableErr)
	case r.DisableErr != nil:
		return fmt.Sprintf("%s: enabled (disable failed: %v)", r.Service, r.DisableErr)
	default:
		return fmt.Sprintf("%s: ok", r.Service)
	}
}

// ReconnectResult lists what Reconnect did to each service.
type ReconnectResult struct {
	Services []ServiceResult
}

// String describes the outcome for every service, separated by semicolons.
func (r ReconnectResult) String() string {
	parts := make([]string, 0, len(r.Services))
	for _, s := range r.Services {
		parts = append(parts, s.String())
	}
	return strings.Join(parts, "; ")
}

// Adapter is what the rest of the program needs from the Bluetooth stack.
type Adapter interface {
	// Devices returns the classic devices Windows knows, sorted by name. The
	// caller decides which of them it wants (Paired, IsAudio).
	Devices() ([]Device, error)

	// Device reads the current state of one paired device. It returns
	// ErrNoRadio or ErrDeviceNotFound when the device cannot be read at all.
	Device(addr Address) (Device, error)

	// Reconnect asks Windows once to connect the audio services of the device.
	// It blocks until Windows has answered, which can take several seconds, and
	// it cannot be interrupted: it switches each service off and on again, and
	// stopping in between would leave that service switched off. A nil error
	// does not mean the device is connected; read it with Device.
	Reconnect(addr Address) (ReconnectResult, error)

	// Disconnect drops the connection to the device, like the Disconnect command
	// of Windows Settings. The device stays paired and its services stay
	// switched on, so it can be connected again. It returns ErrNotConnected when
	// the device has no connection. Whether the device connects again by itself
	// is up to the device. Windows may take a moment to report the change, so
	// read the device afterwards.
	Disconnect(addr Address) error
}

// Restored is a service that Recover switched on again.
type Restored struct {
	Device  string
	Address Address
	Service Service
}

// Recoverer is implemented by adapters that keep a journal of the services they
// switch off.
type Recoverer interface {
	// Recover switches on again every service that an earlier run switched off
	// and did not switch on again, because it was stopped between the two
	// calls. Call it once at start-up, before anything is connected.
	Recover() ([]Restored, error)
}

// Radio describes a local Bluetooth radio, for diagnostics.
type Radio struct {
	Name          string
	Address       Address
	Manufacturer  uint16 // Bluetooth SIG company identifier
	LMPSubversion uint16
}

// RadioLister is implemented by adapters that can describe their radios.
type RadioLister interface {
	Radios() ([]Radio, error)
}
