//go:build windows

package audio

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Core Audio API (mmdeviceapi.h) is COM. It is called through the method
// tables of its interfaces, so the program needs no cgo and no COM package.
var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")
	procPropVariantClear = ole32.NewProc("PropVariantClear")
)

var (
	clsidMMDeviceEnumerator = windows.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = windows.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}

	// Windows has no documented call that sets the default output. This class and
	// its interface IPolicyConfig do it (ADR 0009).
	clsidPolicyConfigClient = windows.GUID{Data1: 0x870AF99C, Data2: 0x171D, Data3: 0x4F9E, Data4: [8]byte{0xAF, 0x0D, 0xE6, 0x3D, 0xF4, 0x0C, 0x2B, 0xC9}}
	iidIPolicyConfig        = windows.GUID{Data1: 0xF8679F50, Data2: 0x850A, Data3: 0x41CF, Data4: [8]byte{0x9C, 0x72, 0x43, 0x0F, 0x29, 0x02, 0x90, 0xC8}}
)

// Positions of the methods in the tables. The first three of every table are
// those of IUnknown. A wrong position would call another method, so these are
// not to be changed without checking the declaration of the interface.
const (
	methodRelease = 2

	enumeratorEnumAudioEndpoints      = 3 // IMMDeviceEnumerator
	enumeratorGetDefaultAudioEndpoint = 4
	collectionGetCount                = 3 // IMMDeviceCollection
	collectionItem                    = 4
	deviceOpenPropertyStore           = 4 // IMMDevice
	deviceGetID                       = 5
	storeGetValue                     = 5 // IPropertyStore

	// IPolicyConfig: GetMixFormat (3), GetDeviceFormat, ResetDeviceFormat,
	// SetDeviceFormat, GetProcessingPeriod, SetProcessingPeriod, GetShareMode,
	// SetShareMode, GetPropertyValue, SetPropertyValue (12), SetDefaultEndpoint
	// (13), SetEndpointVisibility.
	policySetDefaultEndpoint = 13
)

const (
	eRender           = 0    // EDataFlow: outputs
	deviceStateActive = 0x1  // DEVICE_STATE_ACTIVE
	stgmRead          = 0    // STGM_READ
	clsctxAll         = 0x17 // CLSCTX_ALL

	vtUI4    = 19 // VT_UI4
	vtLPWSTR = 31 // VT_LPWSTR

	hrFalse       = 0x00000001 // S_FALSE
	hrChangedMode = 0x80010106 // RPC_E_CHANGED_MODE
	hrNotFound    = 0x80070490 // E_NOTFOUND
)

type propertyKey struct { // PROPERTYKEY
	fmtid windows.GUID
	pid   uint32
}

// propVariant is PROPVARIANT on 64-bit Windows: a type tag and a union of 16 bytes.
type propVariant struct {
	vt    uint16
	_     [3]uint16
	value [2]uintptr
}

// A wrong size would corrupt memory, so the build fails if one does not match.
var (
	_ [unsafe.Sizeof(propertyKey{}) - 20]struct{}
	_ [20 - unsafe.Sizeof(propertyKey{})]struct{}
	_ [unsafe.Sizeof(propVariant{}) - 24]struct{}
	_ [24 - unsafe.Sizeof(propVariant{})]struct{}
)

var (
	keyFriendlyName = propertyKey{ // PKEY_Device_FriendlyName
		fmtid: windows.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD, Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}},
		pid:   14,
	}
	keyFormFactor = propertyKey{ // PKEY_AudioEndpoint_FormFactor
		fmtid: windows.GUID{Data1: 0x1DA5D803, Data2: 0xD492, Data3: 0x4EDD, Data4: [8]byte{0x8C, 0x23, 0xE0, 0xC0, 0xFF, 0xEE, 0x7F, 0x0E}},
		pid:   0,
	}
)

// comError is a failed COM call with its HRESULT.
type comError struct {
	op string
	hr uint32
}

func (e *comError) Error() string {
	return fmt.Sprintf("%s: %s (HRESULT 0x%08X)", e.op, strings.TrimSpace(syscall.Errno(e.hr).Error()), e.hr)
}

func failed(hr uint32) bool { return int32(hr) < 0 }

// comObject is a COM interface pointer: the address of a table of methods.
type comObject struct {
	methods *[16]uintptr
}

// call invokes a method of the object and returns its HRESULT. Pointer
// arguments arrive converted to uintptr; go:uintptrescapes keeps what they
// point to alive and in place until the call returns.
//
//go:uintptrescapes
func (o *comObject) call(method int, args ...uintptr) uint32 {
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(unsafe.Pointer(o)))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(o.methods[method], all...)
	return uint32(r)
}

func (o *comObject) release() { o.call(methodRelease) }

// create makes an instance of a class and returns the interface asked for. The
// caller releases it.
func create(clsid, iid *windows.GUID) (*comObject, error) {
	var obj *comObject
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxAll,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&obj)))
	if failed(uint32(r)) {
		return nil, &comError{"CoCreateInstance", uint32(r)}
	}
	return obj, nil
}

// withCOM runs f on one thread with COM initialized, which is a matter of the
// thread. A thread that has COM initialized in the other mode can be used too.
func withCOM(f func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
	var errno syscall.Errno
	switch {
	case err == nil, errors.As(err, &errno) && uint32(errno) == hrFalse:
		defer windows.CoUninitialize()
	case errors.As(err, &errno) && uint32(errno) == hrChangedMode:
		// COM is initialized already; there is nothing to undo.
	default:
		return fmt.Errorf("CoInitializeEx: %w", err)
	}
	return f()
}

type windowsAdapter struct {
	log *slog.Logger
}

// New returns the adapter for the sound outputs of this PC. log may be nil.
func New(log *slog.Logger) (Adapter, error) {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	for _, p := range []*windows.LazyProc{procCoCreateInstance, procPropVariantClear} {
		if err := p.Find(); err != nil {
			return nil, fmt.Errorf("the audio API of Windows is not available: %w", err)
		}
	}
	return &windowsAdapter{log: log}, nil
}

// State reads the active outputs and the defaults.
func (a *windowsAdapter) State() (State, error) {
	var state State
	err := withCOM(func() error {
		enumerator, err := create(&clsidMMDeviceEnumerator, &iidIMMDeviceEnumerator)
		if err != nil {
			return err
		}
		defer enumerator.release()

		if state.Outputs, err = a.activeOutputs(enumerator); err != nil {
			return err
		}
		for role := RoleConsole; role <= RoleCommunications; role++ {
			if state.Defaults[role], err = defaultID(enumerator, role); err != nil {
				return err
			}
		}
		return nil
	})
	return state, err
}

// SetDefault makes the output the default one of the role.
func (a *windowsAdapter) SetDefault(id string, role Role) error {
	wide, err := windows.UTF16PtrFromString(id)
	if err != nil {
		return fmt.Errorf("the ID of the sound output: %w", err)
	}
	return withCOM(func() error {
		policy, err := create(&clsidPolicyConfigClient, &iidIPolicyConfig)
		if err != nil {
			return fmt.Errorf("%w: %w", ErrUnsupported, err)
		}
		defer policy.release()
		if hr := policy.call(policySetDefaultEndpoint, uintptr(unsafe.Pointer(wide)), uintptr(role)); failed(hr) {
			return &comError{"IPolicyConfig.SetDefaultEndpoint", hr}
		}
		return nil
	})
}

// activeOutputs lists the outputs that can be used now.
func (a *windowsAdapter) activeOutputs(enumerator *comObject) ([]Output, error) {
	var collection *comObject
	if hr := enumerator.call(enumeratorEnumAudioEndpoints, eRender, deviceStateActive, uintptr(unsafe.Pointer(&collection))); failed(hr) {
		return nil, &comError{"IMMDeviceEnumerator.EnumAudioEndpoints", hr}
	}
	defer collection.release()

	var count uint32
	if hr := collection.call(collectionGetCount, uintptr(unsafe.Pointer(&count))); failed(hr) {
		return nil, &comError{"IMMDeviceCollection.GetCount", hr}
	}
	outputs := make([]Output, 0, count)
	for i := range count {
		var device *comObject
		if hr := collection.call(collectionItem, uintptr(i), uintptr(unsafe.Pointer(&device))); failed(hr) {
			return nil, &comError{"IMMDeviceCollection.Item", hr}
		}
		output, err := readOutput(device)
		device.release()
		if err != nil {
			// An output that is being unplugged can fail here; the others are shown.
			a.log.Debug("reading a sound output failed", "err", err)
			continue
		}
		outputs = append(outputs, output)
	}
	SortOutputs(outputs)
	return outputs, nil
}

func readOutput(device *comObject) (Output, error) {
	id, err := deviceID(device)
	if err != nil {
		return Output{}, err
	}
	var store *comObject
	if hr := device.call(deviceOpenPropertyStore, stgmRead, uintptr(unsafe.Pointer(&store))); failed(hr) {
		return Output{}, &comError{"IMMDevice.OpenPropertyStore", hr}
	}
	defer store.release()

	o := Output{ID: id, Name: id, Kind: KindOther}
	err = withProperty(store, &keyFriendlyName, func(v *propVariant) {
		if v.vt != vtLPWSTR {
			return
		}
		if name := windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&v.value[0]))); name != "" {
			o.Name = name
		}
	})
	if err != nil {
		return Output{}, err
	}
	// The kind only picks an icon, so a failure leaves it as "other".
	_ = withProperty(store, &keyFormFactor, func(v *propVariant) {
		if v.vt == vtUI4 {
			o.Kind = kindOfFormFactor(uint32(v.value[0]))
		}
	})
	return o, nil
}

// withProperty reads one property, hands it to read, and frees it.
func withProperty(store *comObject, key *propertyKey, read func(*propVariant)) error {
	var v propVariant
	if hr := store.call(storeGetValue, uintptr(unsafe.Pointer(key)), uintptr(unsafe.Pointer(&v))); failed(hr) {
		return &comError{"IPropertyStore.GetValue", hr}
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&v)))
	read(&v)
	return nil
}

// kindOfFormFactor maps the EndpointFormFactor enumeration of Windows.
func kindOfFormFactor(f uint32) Kind {
	switch f {
	case 1: // Speakers
		return KindSpeaker
	case 3: // Headphones
		return KindHeadphones
	case 5: // Headset
		return KindHeadset
	case 9: // DigitalAudioDisplayDevice
		return KindDisplay
	}
	return KindOther
}

func deviceID(device *comObject) (string, error) {
	var p *uint16
	if hr := device.call(deviceGetID, uintptr(unsafe.Pointer(&p))); failed(hr) {
		return "", &comError{"IMMDevice.GetId", hr}
	}
	defer windows.CoTaskMemFree(unsafe.Pointer(p))
	return windows.UTF16PtrToString(p), nil
}

// defaultID returns the ID of the default output of a role, and an empty one
// when there is none.
func defaultID(enumerator *comObject, role Role) (string, error) {
	var device *comObject
	hr := enumerator.call(enumeratorGetDefaultAudioEndpoint, eRender, uintptr(role), uintptr(unsafe.Pointer(&device)))
	if hr == hrNotFound {
		return "", nil
	}
	if failed(hr) {
		return "", &comError{"IMMDeviceEnumerator.GetDefaultAudioEndpoint", hr}
	}
	defer device.release()
	return deviceID(device)
}
