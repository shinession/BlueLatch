package bt

import "fmt"

// ClassOfDevice is the 24-bit Bluetooth class of device. Bits 8-12 hold the
// major class and bits 2-7 the minor class.
type ClassOfDevice uint32

// MajorAudioVideo is the major device class of headsets, headphones and
// speakers.
const MajorAudioVideo = 0x04

// Major returns the major device class.
func (c ClassOfDevice) Major() uint8 { return uint8(c >> 8 & 0x1f) }

// Minor returns the minor device class, whose meaning depends on the major one.
func (c ClassOfDevice) Minor() uint8 { return uint8(c >> 2 & 0x3f) }

// IsAudioVideo reports whether the major class is Audio/Video.
func (c ClassOfDevice) IsAudioVideo() bool { return c.Major() == MajorAudioVideo }

var majorNames = map[uint8]string{
	0x00: "Miscellaneous",
	0x01: "Computer",
	0x02: "Phone",
	0x03: "LAN/Network access point",
	0x04: "Audio/Video",
	0x05: "Peripheral",
	0x06: "Imaging",
	0x07: "Wearable",
	0x08: "Toy",
	0x09: "Health",
	0x1f: "Uncategorized",
}

var audioVideoMinorNames = map[uint8]string{
	0x00: "Uncategorized",
	0x01: "Wearable Headset",
	0x02: "Hands-free",
	0x04: "Microphone",
	0x05: "Loudspeaker",
	0x06: "Headphones",
	0x07: "Portable Audio",
	0x08: "Car Audio",
	0x09: "Set-top box",
	0x0a: "HiFi Audio",
	0x0b: "VCR",
	0x0c: "Video Camera",
	0x0d: "Camcorder",
	0x0e: "Video Monitor",
	0x0f: "Video Display and Loudspeaker",
	0x10: "Video Conferencing",
	0x12: "Gaming/Toy",
}

// Describe returns a short human-readable type such as "Audio/Video: Headphones".
func (c ClassOfDevice) Describe() string {
	major, ok := majorNames[c.Major()]
	if !ok {
		major = fmt.Sprintf("Class 0x%02X", c.Major())
	}
	if c.IsAudioVideo() {
		if minor, ok := audioVideoMinorNames[c.Minor()]; ok {
			return major + ": " + minor
		}
	}
	return major
}
