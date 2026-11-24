package app

import (
	"sync"
	"time"

	"github.com/shinession/bluelatch/internal/bt"
)

// The addresses of the made-up devices of the demo adapter.
const (
	DemoSpeaker    bt.Address = 0xDE0000000001
	DemoEarbuds    bt.Address = 0xDE0000000002
	DemoHeadphones bt.Address = 0xDE0000000003
)

// NewDemo returns an adapter with three made-up devices, so the window can be
// tried without touching a real device (bluelatch-gui -demo). The speaker is
// connected. The earbuds connect on the third request. The headphones never
// connect, which is what the Cancel button is for. A request takes two seconds
// and cannot be interrupted, like a real one.
func NewDemo() bt.Adapter { return newDemo(2 * time.Second) }

type demo struct {
	delay time.Duration // how long a request takes

	mu      sync.Mutex
	devices []*demoDevice
}

type demoDevice struct {
	dev       bt.Device
	succeedOn int       // the request after which the device connects; 0 means never
	requests  int       // the requests made so far
	upAt      time.Time // when the link comes up after the successful request
}

func newDemo(delay time.Duration) *demo {
	paired := func(d bt.Device) bt.Device {
		d.Remembered, d.Authenticated = true, true
		return d
	}
	return &demo{delay: delay, devices: []*demoDevice{
		{dev: paired(bt.Device{Name: "Demo Speaker", Address: DemoSpeaker, Class: 0x240414, Connected: true,
			Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceAVRCPTarget}})},
		{dev: paired(bt.Device{Name: "Demo Earbuds", Address: DemoEarbuds, Class: 0x248404,
			Services: []bt.Service{bt.ServiceA2DPSink, bt.ServiceHandsfree}}), succeedOn: 3},
		{dev: paired(bt.Device{Name: "Demo Headphones", Address: DemoHeadphones, Class: 0x240418,
			Services: []bt.Service{bt.ServiceA2DPSink}})},
	}}
}

func (x *demoDevice) snapshot() bt.Device {
	d := x.dev
	if !x.upAt.IsZero() && !time.Now().Before(x.upAt) {
		d.Connected = true
	}
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
