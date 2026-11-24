package bt

import (
	"strings"
	"testing"
)

func TestParseAddress(t *testing.T) {
	want := Address(0xAABBCCDDEEFF)
	for _, in := range []string{"AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", "AABBCCDDEEFF", " aabbccddeeff "} {
		got, err := ParseAddress(in)
		if err != nil || got != want {
			t.Errorf("ParseAddress(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
}

func TestParseAddressInvalid(t *testing.T) {
	for _, in := range []string{"", "AA:BB:CC:DD:EE", "AA:BB:CC:DD:EE:FF:00", "GG:BB:CC:DD:EE:FF", "AABBCCDDEEF", "Example Buds", "+ABBCCDDEEFF"} {
		if got, err := ParseAddress(in); err == nil {
			t.Errorf("ParseAddress(%q) = %v, want an error", in, got)
		}
	}
}

func TestAddressString(t *testing.T) {
	for addr, want := range map[Address]string{
		0xAABBCCDDEEFF: "AA:BB:CC:DD:EE:FF",
		0x0000000000AB: "00:00:00:00:00:AB",
	} {
		if got := addr.String(); got != want {
			t.Errorf("Address(%#x).String() = %q, want %q", uint64(addr), got, want)
		}
	}
}

func TestClassOfDevice(t *testing.T) {
	tests := []struct {
		class ClassOfDevice
		audio bool
		text  string
	}{
		{0x240404, true, "Audio/Video: Wearable Headset"},
		{0x248404, true, "Audio/Video: Wearable Headset"}, // other service class bits
		{0x240414, true, "Audio/Video: Loudspeaker"},
		{0x240418, true, "Audio/Video: Headphones"},
		{0x240400, true, "Audio/Video: Uncategorized"},
		{0x24047C, true, "Audio/Video"}, // a minor class without a name
		{0x002580, false, "Peripheral"},
		{0x5A020C, false, "Phone"},
		{0x000A00, false, "Class 0x0A"},
	}
	for _, tt := range tests {
		if got := tt.class.IsAudioVideo(); got != tt.audio {
			t.Errorf("ClassOfDevice(%#06x).IsAudioVideo() = %v, want %v", uint32(tt.class), got, tt.audio)
		}
		if got := tt.class.Describe(); got != tt.text {
			t.Errorf("ClassOfDevice(%#06x).Describe() = %q, want %q", uint32(tt.class), got, tt.text)
		}
	}
}

func TestServiceString(t *testing.T) {
	if got := ServiceA2DPSink.String(); got != "A2DP Sink" {
		t.Errorf("ServiceA2DPSink.String() = %q", got)
	}
	if got := Service(0x1801).String(); got != "0x1801" {
		t.Errorf("Service(0x1801).String() = %q", got)
	}
}

func TestReconnectServicesIsACopy(t *testing.T) {
	got := ReconnectServices()
	if len(got) == 0 || got[0] != ServiceA2DPSink {
		t.Fatalf("ReconnectServices() = %v, want the stereo link first", got)
	}
	got[0] = ServiceHeadset
	if ReconnectServices()[0] != ServiceA2DPSink {
		t.Error("changing the returned slice changed the services Reconnect uses")
	}
}

func TestDeviceIsAudio(t *testing.T) {
	tests := []struct {
		name string
		dev  Device
		want bool
	}{
		{"audio class", Device{Class: 0x240404}, true},
		{"peripheral with A2DP", Device{Class: 0x002580, Services: []Service{ServiceA2DPSink}}, true},
		{"peripheral with hands-free", Device{Class: 0x002580, Services: []Service{ServiceHandsfree}}, true},
		{"peripheral with headset", Device{Class: 0x002580, Services: []Service{ServiceHeadset}}, true},
		{"only a remote control", Device{Class: 0x002580, Services: []Service{ServiceAVRCPTarget}}, false},
		{"nothing known", Device{}, false},
	}
	for _, tt := range tests {
		if got := tt.dev.IsAudio(); got != tt.want {
			t.Errorf("%s: IsAudio() = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDevicePaired(t *testing.T) {
	tests := []struct {
		dev  Device
		want bool
	}{
		{Device{Authenticated: true}, true},
		{Device{Remembered: true}, true},
		{Device{Connected: true}, false},
		{Device{Authenticated: true, Connected: true}, true},
	}
	for _, tt := range tests {
		if got := tt.dev.Paired(); got != tt.want {
			t.Errorf("%+v.Paired() = %v, want %v", tt.dev, got, tt.want)
		}
	}
}

func TestDeviceDisplayName(t *testing.T) {
	if got := (Device{Name: "Buds", Address: 1}).DisplayName(); got != "Buds" {
		t.Errorf("DisplayName() = %q, want the name", got)
	}
	if got := (Device{Name: "  ", Address: 0xAB}).DisplayName(); got != "00:00:00:00:00:AB" {
		t.Errorf("DisplayName() of an unnamed device = %q, want the address", got)
	}
}

func TestSortDevices(t *testing.T) {
	devices := []Device{
		{Name: "b", Address: 1},
		{Name: "A", Address: 3},
		{Name: "a", Address: 2},
	}
	SortDevices(devices)
	var got []Address
	for _, d := range devices {
		got = append(got, d.Address)
	}
	if want := []Address{2, 3, 1}; got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("order after SortDevices = %v, want %v", got, want)
	}
}

type errString string

func (e errString) Error() string { return string(e) }

func TestReconnectResultString(t *testing.T) {
	res := ReconnectResult{Services: []ServiceResult{
		{Service: ServiceA2DPSink},
		{Service: ServiceHandsfree, EnableErr: errString("busy")},
		{Service: ServiceHeadset, DisableErr: errString("denied")},
	}}
	want := "A2DP Sink: ok; Hands-Free: enable failed: busy; Headset: enabled (disable failed: denied)"
	if got := res.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestFind(t *testing.T) {
	devices := []Device{
		{Name: "Example Buds", Address: 0xAABBCCDDEE02},
		{Name: "Example Airwave", Address: 0xAABBCCDDEE03},
		{Name: "Studio One", Address: 0xAABBCCDDEE11},
		{Name: "Studio Two", Address: 0xAABBCCDDEE12},
		{Name: "Air", Address: 0x111111111111},
	}
	tests := []struct {
		query   string
		want    Address
		wantErr string // a part of the error, empty for success
	}{
		{"AA:BB:CC:DD:EE:02", 0xAABBCCDDEE02, ""},
		{"aabbccddee02", 0xAABBCCDDEE02, ""},
		{"studio one", 0xAABBCCDDEE11, ""},
		{"  BUDS  ", 0xAABBCCDDEE02, ""},
		{"air", 0x111111111111, ""}, // a whole name beats a part of "Example Airwave"
		{"studio", 0, "matches several devices"},
		{"example", 0, "matches several devices"},
		{"sony", 0, `no device matches "sony"`},
		{"00:00:00:00:00:01", 0, "no device has the address 00:00:00:00:00:01"},
		{"", 0, "is empty"},
	}
	for _, tt := range tests {
		got, err := Find(devices, tt.query)
		if tt.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Find(%q) error = %v, want one containing %q", tt.query, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got.Address != tt.want {
			t.Errorf("Find(%q) = %v, %v; want address %v", tt.query, got.Address, err, tt.want)
		}
	}
}

func TestFindWithoutDevices(t *testing.T) {
	_, err := Find(nil, "buds")
	if err == nil || !strings.Contains(err.Error(), "there are no devices") {
		t.Errorf("Find(nil, ...) error = %v, want one saying there are no devices", err)
	}
}
