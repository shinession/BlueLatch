//go:build windows

package bt

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestServiceGUIDs(t *testing.T) {
	want, err := windows.GUIDFromString("{0000110B-0000-1000-8000-00805F9B34FB}")
	if err != nil {
		t.Fatal(err)
	}
	if got := guidFromService(ServiceA2DPSink); got != want {
		t.Errorf("guidFromService(ServiceA2DPSink) = %v, want %v", got, want)
	}
	if s, ok := serviceFromGUID(want); !ok || s != ServiceA2DPSink {
		t.Errorf("serviceFromGUID(%v) = %v, %v; want the A2DP sink", want, s, ok)
	}

	vendor, err := windows.GUIDFromString("{12345678-0000-1000-8000-00805F9B34FB}") // a 32-bit UUID
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := serviceFromGUID(vendor); ok {
		t.Errorf("serviceFromGUID(%v) = %v, want it to be rejected", vendor, s)
	}
	other, err := windows.GUIDFromString("{0000110B-0000-1000-8000-00805F9B34FC}") // not the Bluetooth base
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := serviceFromGUID(other); ok {
		t.Errorf("serviceFromGUID(%v) = %v, want it to be rejected", other, s)
	}
}

func TestSystemTime(t *testing.T) {
	if got := systemTime(windows.Systemtime{}); !got.IsZero() {
		t.Errorf("systemTime(zero) = %v, want the zero time", got)
	}
	got := systemTime(windows.Systemtime{Year: 2026, Month: 11, Day: 24, Hour: 18, Minute: 21, Second: 57, Milliseconds: 250})
	want := time.Date(2026, 11, 24, 18, 21, 57, 250*int(time.Millisecond), time.Local)
	if !got.Equal(want) {
		t.Errorf("systemTime = %v, want %v", got, want)
	}
}

func TestWinError(t *testing.T) {
	err := codeError("BluetoothSetServiceState", uintptr(windows.ERROR_SEM_TIMEOUT))
	if err == nil {
		t.Fatal("codeError returned nil for a non-zero code")
	}
	if !errors.Is(err, windows.ERROR_SEM_TIMEOUT) {
		t.Errorf("errors.Is(%v, ERROR_SEM_TIMEOUT) = false", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "BluetoothSetServiceState") || !strings.Contains(msg, "error 121") {
		t.Errorf("Error() = %q, want the call and the code", msg)
	}
	if codeError("x", 0) != nil {
		t.Error("codeError(0) is not nil")
	}
	if got := lastError("x", syscall.Errno(0)); got == nil || !strings.Contains(got.Error(), "without an error code") {
		t.Errorf("lastError with errno 0 = %v", got)
	}
}

func TestDisconnectRequest(t *testing.T) {
	// CTL_CODE(FILE_DEVICE_BLUETOOTH, 3, METHOD_BUFFERED, FILE_ANY_ACCESS).
	if ioctlBthDisconnectDevice != 0x0041000C {
		t.Errorf("ioctlBthDisconnectDevice = %#x, want 0x0041000C", uint32(ioctlBthDisconnectDevice))
	}
	if !notConnected(windows.ERROR_DEVICE_NOT_CONNECTED) {
		t.Error("ERROR_DEVICE_NOT_CONNECTED is not taken for a device that is not connected")
	}
	wrapped := &winError{op: "IOCTL_BTH_DISCONNECT_DEVICE", errno: windows.ERROR_DEVICE_NOT_CONNECTED}
	if !notConnected(wrapped) {
		t.Error("a winError with ERROR_DEVICE_NOT_CONNECTED is not taken for a device that is not connected")
	}
	for _, other := range []syscall.Errno{windows.ERROR_ACCESS_DENIED, windows.ERROR_NOT_FOUND, windows.ERROR_NOT_CONNECTED, windows.ERROR_INVALID_PARAMETER} {
		if notConnected(other) {
			t.Errorf("error %d is taken for a device that is not connected, but it may be a real failure", uintptr(other))
		}
	}
}

// TestLiveDevices reads the real device list. It is read-only, but it needs a
// Bluetooth radio, so it only runs when BLUELATCH_LIVE=1.
func TestLiveDevices(t *testing.T) {
	if os.Getenv("BLUELATCH_LIVE") != "1" {
		t.Skip("set BLUELATCH_LIVE=1 to read the real Bluetooth devices")
	}
	adapter, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := adapter.Devices()
	if errors.Is(err, ErrNoRadio) {
		t.Skip(err)
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		t.Logf("%s %s connected=%v paired=%v audio=%v services=%v", d.DisplayName(), d.Address, d.Connected, d.Paired(), d.IsAudio(), d.Services)
		again, err := adapter.Device(d.Address)
		if err != nil {
			t.Errorf("Device(%s): %v", d.Address, err)
			continue
		}
		if again.Address != d.Address || again.Name != d.Name {
			t.Errorf("Device(%s) = %s %s, want the same device", d.Address, again.Name, again.Address)
		}
	}
}
