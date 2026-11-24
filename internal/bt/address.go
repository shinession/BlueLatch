// Package bt is the part of the Windows Bluetooth stack that BlueLatch needs:
// listing the paired classic audio devices, reading whether one is connected,
// and asking Windows to connect it once. The types and the Adapter interface
// are platform independent; the Windows implementation is in adapter_windows.go.
package bt

import (
	"fmt"
	"strconv"
	"strings"
)

// Address is a 48-bit Bluetooth device address held in the low six bytes of a
// uint64. It has the layout of the Win32 BLUETOOTH_ADDRESS union, so the first
// octet of the printed form is the most significant one.
type Address uint64

// String returns the address as AA:BB:CC:DD:EE:FF.
func (a Address) String() string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		byte(a>>40), byte(a>>32), byte(a>>24), byte(a>>16), byte(a>>8), byte(a))
}

// ParseAddress reads an address written as AA:BB:CC:DD:EE:FF, AA-BB-CC-DD-EE-FF
// or AABBCCDDEEFF, in any case.
func ParseAddress(s string) (Address, error) {
	clean := strings.NewReplacer(":", "", "-", "").Replace(strings.TrimSpace(s))
	if len(clean) != 12 {
		return 0, fmt.Errorf("bluetooth address %q: want 12 hex digits", s)
	}
	v, err := strconv.ParseUint(clean, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("bluetooth address %q: want 12 hex digits", s)
	}
	return Address(v), nil
}
