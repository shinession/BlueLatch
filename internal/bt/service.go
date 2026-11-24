package bt

import "fmt"

// Service is a 16-bit Bluetooth SIG service class ID. The full UUID is
// 0000xxxx-0000-1000-8000-00805F9B34FB.
type Service uint16

// The services of a classic audio device that matter here.
const (
	ServiceHeadset         Service = 0x1108
	ServiceA2DPSource      Service = 0x110A
	ServiceA2DPSink        Service = 0x110B
	ServiceAVRCPTarget     Service = 0x110C
	ServiceAVRCPRemote     Service = 0x110E
	ServiceAVRCPController Service = 0x110F
	ServiceHandsfree       Service = 0x111E
	ServiceHandsfreeAG     Service = 0x111F
)

var serviceNames = map[Service]string{
	ServiceHeadset:         "Headset",
	ServiceA2DPSource:      "A2DP Source",
	ServiceA2DPSink:        "A2DP Sink",
	ServiceAVRCPTarget:     "AVRCP Target",
	ServiceAVRCPRemote:     "AVRCP Remote",
	ServiceAVRCPController: "AVRCP Controller",
	ServiceHandsfree:       "Hands-Free",
	ServiceHandsfreeAG:     "Hands-Free AG",
}

// String returns the name of the service, or its hex ID when it has none here.
func (s Service) String() string {
	if name, ok := serviceNames[s]; ok {
		return name
	}
	return fmt.Sprintf("0x%04X", uint16(s))
}

// audioServices are the services that make a device an audio device.
var audioServices = []Service{ServiceA2DPSink, ServiceHandsfree, ServiceHeadset}

// ReconnectServices returns the services that Reconnect switches off and on, in
// the order it handles them: the stereo audio link first, then the voice links.
func ReconnectServices() []Service {
	return append([]Service(nil), audioServices...)
}
