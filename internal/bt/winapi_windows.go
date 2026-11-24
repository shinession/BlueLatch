//go:build windows

package bt

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Win32 Bluetooth API (bluetoothapis.h) lives in bthprops.cpl. It covers
// classic (BR/EDR) devices only, which is what BlueLatch handles. It is called
// directly through syscalls, so the program needs no cgo.
var bthprops = windows.NewLazySystemDLL("bthprops.cpl")

var (
	procFindFirstRadio             = bthprops.NewProc("BluetoothFindFirstRadio")
	procFindNextRadio              = bthprops.NewProc("BluetoothFindNextRadio")
	procFindRadioClose             = bthprops.NewProc("BluetoothFindRadioClose")
	procGetRadioInfo               = bthprops.NewProc("BluetoothGetRadioInfo")
	procFindFirstDevice            = bthprops.NewProc("BluetoothFindFirstDevice")
	procFindNextDevice             = bthprops.NewProc("BluetoothFindNextDevice")
	procFindDeviceClose            = bthprops.NewProc("BluetoothFindDeviceClose")
	procGetDeviceInfo              = bthprops.NewProc("BluetoothGetDeviceInfo")
	procEnumerateInstalledServices = bthprops.NewProc("BluetoothEnumerateInstalledServices")
	procSetServiceState            = bthprops.NewProc("BluetoothSetServiceState")
)

// requiredProcs are checked once by New, so a missing export is reported as an
// error instead of panicking inside LazyProc.Call.
var requiredProcs = []*windows.LazyProc{
	procFindFirstRadio, procFindNextRadio, procFindRadioClose, procGetRadioInfo,
	procFindFirstDevice, procFindNextDevice, procFindDeviceClose, procGetDeviceInfo,
	procEnumerateInstalledServices, procSetServiceState,
}

const (
	maxNameSize = 248 // BLUETOOTH_MAX_NAME_SIZE, in UTF-16 units

	serviceDisable = 0 // BLUETOOTH_SERVICE_DISABLE
	serviceEnable  = 1 // BLUETOOTH_SERVICE_ENABLE
)

// The structs below mirror bluetoothapis.h on 64-bit Windows. A wrong size
// would corrupt memory, so the build fails if one does not match.

type findRadioParams struct { // BLUETOOTH_FIND_RADIO_PARAMS
	size uint32
}

type radioInfo struct { // BLUETOOTH_RADIO_INFO
	size          uint32
	address       uint64
	name          [maxNameSize]uint16
	classOfDevice uint32
	lmpSubversion uint16
	manufacturer  uint16
}

type deviceSearchParams struct { // BLUETOOTH_DEVICE_SEARCH_PARAMS
	size                uint32
	returnAuthenticated int32
	returnRemembered    int32
	returnUnknown       int32
	returnConnected     int32
	issueInquiry        int32
	timeoutMultiplier   uint8
	radio               windows.Handle
}

type deviceInfo struct { // BLUETOOTH_DEVICE_INFO
	size          uint32
	address       uint64
	classOfDevice uint32
	connected     int32
	remembered    int32
	authenticated int32
	lastSeen      windows.Systemtime
	lastUsed      windows.Systemtime
	name          [maxNameSize]uint16
}

const (
	findRadioParamsSize = uint32(unsafe.Sizeof(findRadioParams{}))
	radioInfoSize       = uint32(unsafe.Sizeof(radioInfo{}))
	deviceInfoSize      = uint32(unsafe.Sizeof(deviceInfo{}))
)

var (
	_ [unsafe.Sizeof(findRadioParams{}) - 4]struct{}
	_ [4 - unsafe.Sizeof(findRadioParams{})]struct{}
	_ [unsafe.Sizeof(radioInfo{}) - 520]struct{}
	_ [520 - unsafe.Sizeof(radioInfo{})]struct{}
	_ [unsafe.Sizeof(deviceSearchParams{}) - 40]struct{}
	_ [40 - unsafe.Sizeof(deviceSearchParams{})]struct{}
	_ [unsafe.Sizeof(deviceInfo{}) - 560]struct{}
	_ [560 - unsafe.Sizeof(deviceInfo{})]struct{}
)

// winError is a failed Win32 call together with its error code.
type winError struct {
	op    string
	errno syscall.Errno
}

func (e *winError) Error() string {
	return fmt.Sprintf("%s: %s (error %d)", e.op, strings.TrimSpace(e.errno.Error()), uintptr(e.errno))
}

func (e *winError) Unwrap() error { return e.errno }

// lastError turns the error LazyProc.Call returns (the thread's last error code,
// never nil) into a winError, for calls that signal failure through their result.
func lastError(op string, e error) error {
	var errno syscall.Errno
	if !errors.As(e, &errno) || errno == 0 {
		return fmt.Errorf("%s: failed without an error code", op)
	}
	return &winError{op: op, errno: errno}
}

// codeError turns the DWORD result of a call that returns its error code
// directly into a winError. Zero means success.
func codeError(op string, code uintptr) error {
	if code == 0 {
		return nil
	}
	return &winError{op: op, errno: syscall.Errno(code)}
}

// radio is an open handle to a local Bluetooth radio.
type radio struct {
	handle windows.Handle
	info   radioInfo
}

// openRadios opens every local Bluetooth radio. The caller must pass the result
// to closeRadios. It returns ErrNoRadio when there is none, which is also what
// Windows reports while Bluetooth is turned off.
func openRadios() ([]radio, error) {
	params := findRadioParams{size: findRadioParamsSize}
	var handle windows.Handle
	find, _, e := procFindFirstRadio.Call(uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&handle)))
	if find == 0 {
		if errors.Is(e, windows.ERROR_NO_MORE_ITEMS) {
			return nil, ErrNoRadio
		}
		return nil, lastError("BluetoothFindFirstRadio", e)
	}
	defer procFindRadioClose.Call(find)

	var radios []radio
	for {
		rd := radio{handle: handle}
		rd.info.size = radioInfoSize
		// The radio information is only used for diagnostics, so a failure here
		// leaves it empty instead of failing the whole call.
		procGetRadioInfo.Call(uintptr(handle), uintptr(unsafe.Pointer(&rd.info)))
		radios = append(radios, rd)

		var next windows.Handle
		ok, _, _ := procFindNextRadio.Call(find, uintptr(unsafe.Pointer(&next)))
		if ok == 0 {
			return radios, nil
		}
		handle = next
	}
}

func closeRadios(radios []radio) {
	for _, rd := range radios {
		windows.CloseHandle(rd.handle)
	}
}

// enumerateDevices lists the devices of one radio that are authenticated,
// remembered or connected. It does not start an inquiry, so it returns at once.
func enumerateDevices(rd radio) ([]deviceInfo, error) {
	params := deviceSearchParams{
		returnAuthenticated: 1,
		returnRemembered:    1,
		returnConnected:     1,
		radio:               rd.handle,
	}
	params.size = uint32(unsafe.Sizeof(params))

	info := deviceInfo{size: deviceInfoSize}
	find, _, e := procFindFirstDevice.Call(uintptr(unsafe.Pointer(&params)), uintptr(unsafe.Pointer(&info)))
	if find == 0 {
		if errors.Is(e, windows.ERROR_NO_MORE_ITEMS) {
			return nil, nil
		}
		return nil, lastError("BluetoothFindFirstDevice", e)
	}
	defer procFindDeviceClose.Call(find)

	var devices []deviceInfo
	for {
		devices = append(devices, info)
		info = deviceInfo{size: deviceInfoSize}
		ok, _, e := procFindNextDevice.Call(find, uintptr(unsafe.Pointer(&info)))
		if ok == 0 {
			if errors.Is(e, windows.ERROR_NO_MORE_ITEMS) {
				return devices, nil
			}
			return nil, lastError("BluetoothFindNextDevice", e)
		}
	}
}

// getDeviceInfo reads the current state of the device with the given address as
// that radio sees it.
func getDeviceInfo(rd radio, addr Address) (deviceInfo, error) {
	info := deviceInfo{size: deviceInfoSize, address: uint64(addr)}
	code, _, _ := procGetDeviceInfo.Call(uintptr(rd.handle), uintptr(unsafe.Pointer(&info)))
	if err := codeError("BluetoothGetDeviceInfo", code); err != nil {
		return deviceInfo{}, err
	}
	return info, nil
}

// installedServices returns the standard (SIG) services for which Windows has
// installed a driver for the device. Vendor specific 128-bit services are left out.
func installedServices(rd radio, info *deviceInfo) ([]Service, error) {
	const op = "BluetoothEnumerateInstalledServices"
	n := uint32(16)
	for range 3 {
		guids := make([]windows.GUID, n)
		count := n
		code, _, _ := procEnumerateInstalledServices.Call(
			uintptr(rd.handle), uintptr(unsafe.Pointer(info)),
			uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&guids[0])))
		switch syscall.Errno(code) {
		case 0:
			var services []Service
			for _, g := range guids[:min(count, n)] {
				if s, ok := serviceFromGUID(g); ok {
					services = append(services, s)
				}
			}
			return services, nil
		case windows.ERROR_MORE_DATA:
			n = max(count, 2*n)
		default:
			return nil, codeError(op, code)
		}
	}
	return nil, fmt.Errorf("%s: the list of services kept growing", op)
}

// setServiceState switches one service of the device off (serviceDisable) or on
// (serviceEnable). Switching a service on is what makes Windows connect it.
func setServiceState(rd radio, info *deviceInfo, s Service, flags uintptr) error {
	guid := guidFromService(s)
	code, _, _ := procSetServiceState.Call(
		uintptr(rd.handle), uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&guid)), flags)
	return codeError("BluetoothSetServiceState", code)
}

// sigBaseData4 is the last eight bytes of the Bluetooth base UUID
// 0000xxxx-0000-1000-8000-00805F9B34FB.
var sigBaseData4 = [8]byte{0x80, 0x00, 0x00, 0x80, 0x5F, 0x9B, 0x34, 0xFB}

func serviceFromGUID(g windows.GUID) (Service, bool) {
	if g.Data1>>16 != 0 || g.Data2 != 0 || g.Data3 != 0x1000 || g.Data4 != sigBaseData4 {
		return 0, false
	}
	return Service(g.Data1), true
}

func guidFromService(s Service) windows.GUID {
	return windows.GUID{Data1: uint32(s), Data3: 0x1000, Data4: sigBaseData4}
}

// systemTime converts a SYSTEMTIME from the Bluetooth API. Windows leaves it
// zero when it has no value.
func systemTime(st windows.Systemtime) time.Time {
	if st.Year == 0 {
		return time.Time{}
	}
	return time.Date(int(st.Year), time.Month(st.Month), int(st.Day),
		int(st.Hour), int(st.Minute), int(st.Second), int(st.Milliseconds)*int(time.Millisecond), time.Local)
}
