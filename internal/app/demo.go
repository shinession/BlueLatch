package app

import (
	"slices"
	"sync"
	"time"

	"github.com/shinession/bluelatch/internal/audio"
	"github.com/shinession/bluelatch/internal/bt"
)

// The addresses of the made-up devices of the demo adapter.
const (
	DemoSpeaker    bt.Address = 0xDE0000000001
	DemoEarbuds    bt.Address = 0xDE0000000002
	DemoHeadphones bt.Address = 0xDE0000000003
)

// DemoMonitor is the ID of the made-up output that is there all the time. The
// other made-up outputs belong to the devices.
const DemoMonitor = "demo-output-monitor"

// NewDemo returns an adapter with three made-up devices and made-up sound
// outputs, so the window can be tried without touching a real device (bluelatch
// window -demo). The speaker is connected and is the default output. The earbuds
// connect on the third request. The headphones never connect, which is what the
// Cancel button is for. Any of them can be disconnected, and the earbuds then
// need three requests again. A reconnect request takes two seconds and cannot be
// interrupted, like a real one; a disconnect request takes one. A monitor is
// always an output; a device is one while it is connected, and the monitor takes
// over when the default output goes away.
func NewDemo() (bt.Adapter, audio.Adapter) {
	d := newDemo(2 * time.Second)
	return d, d
}

type demo struct {
	delay time.Duration // how long a reconnect request takes

	mu       sync.Mutex
	devices  []*demoDevice
	defaults audio.Defaults
}

type demoDevice struct {
	dev       bt.Device
	connected bool
	succeedOn int       // the request after which the device connects; 0 means never
	requests  int       // the requests made since the device was last disconnected
	upAt      time.Time // when the link comes up after the successful request
}

func newDemo(delay time.Duration) *demo {
	paired := func(d bt.Device) bt.Device {
		d.Remembered, d.Authenticated = true, true
		return d
	}
	d := &demo{delay: delay, devices: []*demoDevice{
		{dev: paired(bt.Device{Name: "Demo Speaker", Address: DemoSpeaker, Class: 0x240414,
			Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceAVRCPTarget}}), connected: true},
		{dev: paired(bt.Device{Name: "Demo Earbuds", Address: DemoEarbuds, Class: 0x248404,
			Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceHandsfree}}), succeedOn: 3},
		{dev: paired(bt.Device{Name: "Demo Headphones", Address: DemoHeadphones, Class: 0x240418,
			Services: []bt.Service{bt.ServiceA2DPSink}})},
	}}
	speaker := demoOutput(d.devices[0].dev).ID
	d.defaults = audio.Defaults{speaker, speaker, speaker}
	return d
}

// demoOutput is the sound output of a made-up device, named the way Windows names them.
func demoOutput(dev bt.Device) audio.Output {
	kind, what := audio.KindHeadphones, "Headphones"
	if dev.Class.Minor() == 0x05 { // loudspeaker
		kind, what = audio.KindSpeaker, "Speakers"
	}
	return audio.Output{ID: "demo-output-" + dev.Address.String(), Name: what + " (" + dev.Name + ")", Kind: kind}
}

// outputs returns the sound outputs as Windows would list them now. The caller
// holds the lock.
func (d *demo) outputs() []audio.Output {
	outputs := []audio.Output{{ID: DemoMonitor, Name: "Demo Monitor (Demo Display Audio)", Kind: audio.KindDisplay}}
	for _, x := range d.devices {
		if x.snapshot().Connected {
			outputs = append(outputs, demoOutput(x.dev))
		}
	}
	audio.SortOutputs(outputs)
	return outputs
}

// State reads the made-up outputs. A default that has gone is taken over by the
// monitor.
func (d *demo) State() (audio.State, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	outputs := d.outputs()
	for role, id := range d.defaults {
		if !slices.ContainsFunc(outputs, func(o audio.Output) bool { return o.ID == id }) {
			d.defaults[role] = DemoMonitor
		}
	}
	return audio.State{Outputs: outputs, Defaults: d.defaults}, nil
}

// SetDefault makes a made-up output the default one of the role.
func (d *demo) SetDefault(id string, role audio.Role) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !slices.ContainsFunc(d.outputs(), func(o audio.Output) bool { return o.ID == id }) {
		return audio.ErrNotFound
	}
	d.defaults[role] = id
	return nil
}

// snapshot returns the device as Windows would report it now. The caller holds
// the lock.
func (x *demoDevice) snapshot() bt.Device {
	if !x.upAt.IsZero() && !time.Now().Before(x.upAt) {
		x.connected, x.upAt = true, time.Time{}
	}
	d := x.dev
	d.Connected = x.connected
	return d
}

func (d *demo) find(addr bt.Address) *demoDevice {
	for _, x := range d.devices {
		if x.dev.Address == addr {
			return x
		}
	}
	return nil
}

func (d *demo) Devices() ([]bt.Device, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]bt.Device, 0, len(d.devices))
	for _, x := range d.devices {
		out = append(out, x.snapshot())
	}
	bt.SortDevices(out)
	return out, nil
}

func (d *demo) Device(addr bt.Address) (bt.Device, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	x := d.find(addr)
	if x == nil {
		return bt.Device{}, bt.ErrDeviceNotFound
	}
	return x.snapshot(), nil
}

func (d *demo) Reconnect(addr bt.Address) (bt.ReconnectResult, error) {
	d.mu.Lock()
	x := d.find(addr)
	d.mu.Unlock()
	if x == nil {
		return bt.ReconnectResult{}, bt.ErrDeviceNotFound
	}

	time.Sleep(d.delay)

	d.mu.Lock()
	defer d.mu.Unlock()
	x.requests++
	if x.succeedOn > 0 && x.requests == x.succeedOn {
		x.upAt = time.Now().Add(d.delay / 2)
	}
	var res bt.ReconnectResult
	for _, s := range bt.ReconnectServices() {
		if x.dev.HasService(s) {
			res.Services = append(res.Services, bt.ServiceResult{Service: s})
		}
	}
	return res, nil
}

func (d *demo) Disconnect(addr bt.Address) error {
	d.mu.Lock()
	x := d.find(addr)
	d.mu.Unlock()
	if x == nil {
		return bt.ErrDeviceNotFound
	}

	time.Sleep(d.delay / 2)

	d.mu.Lock()
	defer d.mu.Unlock()
	if !x.snapshot().Connected {
		return bt.ErrNotConnected
	}
	x.connected, x.upAt, x.requests = false, time.Time{}, 0
	return nil
}
