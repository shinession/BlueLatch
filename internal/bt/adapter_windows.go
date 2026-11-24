//go:build windows

package bt

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/windows"

	"github.com/shinession/bluelatch/internal/appdir"
)

// defaultToggleGap is the pause between switching a service off and on again,
// so the stack has torn the old link down before it is asked to build a new one.
const defaultToggleGap = 300 * time.Millisecond

type winAdapter struct {
	log       *slog.Logger
	toggleGap time.Duration
	journal   *journal // nil when the folder of the program is not available
}

// New returns the Windows implementation of Adapter. A nil logger discards its
// diagnostics. It fails when the Bluetooth API is not present on this system.
//
// The adapter implements Recoverer: it keeps a journal of the services it
// switches off in %LocalAppData%\BlueLatch\pending, so that a later run can
// switch them on again if this one is stopped in between.
func New(log *slog.Logger) (Adapter, error) {
	for _, p := range requiredProcs {
		if err := p.Find(); err != nil {
			return nil, fmt.Errorf("the Windows Bluetooth API is not available: %w", err)
		}
	}
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	a := &winAdapter{log: log, toggleGap: defaultToggleGap}
	if dir, err := appdir.Dir(); err != nil {
		log.Warn("the journal of service switches is not available", "err", err)
	} else {
		a.journal = &journal{dir: filepath.Join(dir, "pending")}
	}
	return a, nil
}

func (a *winAdapter) Devices() ([]Device, error) {
	radios, err := openRadios()
	if err != nil {
		return nil, err
	}
	defer closeRadios(radios)

	var devices []Device
	index := make(map[Address]int)
	for _, rd := range radios {
		infos, err := enumerateDevices(rd)
		if err != nil {
			return nil, err
		}
		for i := range infos {
			dev := a.newDevice(rd, &infos[i])
			if at, seen := index[dev.Address]; seen {
				devices[at].Connected = devices[at].Connected || dev.Connected
				continue
			}
			index[dev.Address] = len(devices)
			devices = append(devices, dev)
		}
	}
	SortDevices(devices)
	return devices, nil
}

func (a *winAdapter) Device(addr Address) (Device, error) {
	radios, err := openRadios()
	if err != nil {
		return Device{}, err
	}
	defer closeRadios(radios)

	rd, info, err := findDevice(radios, addr)
	if err != nil {
		return Device{}, err
	}
	return a.newDevice(rd, &info), nil
}

func (a *winAdapter) Reconnect(addr Address) (ReconnectResult, error) {
	radios, err := openRadios()
	if err != nil {
		return ReconnectResult{}, err
	}
	defer closeRadios(radios)

	rd, info, err := findDevice(radios, addr)
	if err != nil {
		return ReconnectResult{}, err
	}
	installed, err := installedServices(rd, &info)
	if err != nil {
		return ReconnectResult{}, err
	}

	var targets []Service
	for _, s := range audioServices { // keeps the order of audioServices
		if slices.Contains(installed, s) {
			targets = append(targets, s)
		}
	}
	if len(targets) == 0 {
		return ReconnectResult{}, ErrNoAudioServices
	}

	name := windows.UTF16ToString(info.name[:])
	var result ReconnectResult
	enabled := 0
	for _, s := range targets {
		r := ServiceResult{Service: s}
		// Logged before the call: when the process dies between the two calls,
		// the log says which service may have been left switched off.
		a.log.Info("switching the service off", "device", name, "service", s.String())
		a.journalBegin(name, addr, s)
		r.DisableErr = setServiceState(rd, &info, s, serviceDisable)
		a.debugService("the service was switched off", s, r.DisableErr, rd, &info)
		time.Sleep(a.toggleGap)
		// Always switch the service back on, even when switching it off failed:
		// this call is what asks Windows to connect, and a service must never be
		// left switched off.
		r.EnableErr = setServiceState(rd, &info, s, serviceEnable)
		a.debugService("the service was switched on", s, r.EnableErr, rd, &info)
		if r.EnableErr == nil {
			enabled++
			a.journalEnd(addr, s) // when enabling failed, the record stays for Recover
		}
		result.Services = append(result.Services, r)
	}
	if enabled == 0 {
		return result, fmt.Errorf("no audio service could be enabled: %s", result)
	}
	return result, nil
}

// Radios implements RadioLister.
func (a *winAdapter) Radios() ([]Radio, error) {
	radios, err := openRadios()
	if err != nil {
		return nil, err
	}
	defer closeRadios(radios)

	out := make([]Radio, 0, len(radios))
	for _, rd := range radios {
		out = append(out, Radio{
			Name:          windows.UTF16ToString(rd.info.name[:]),
			Address:       Address(rd.info.address),
			Manufacturer:  rd.info.manufacturer,
			LMPSubversion: rd.info.lmpSubversion,
		})
	}
	return out, nil
}

// debugService logs the outcome of one switch of a service together with the
// services Windows lists as installed at that moment. It shows what the list does
// while a service is switched off and on again. It does nothing unless debug
// logging is on, so the list is not read for nothing.
func (a *winAdapter) debugService(msg string, s Service, err error, rd radio, info *deviceInfo) {
	if !a.log.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	a.log.Debug(msg, "service", s.String(), "err", err, "installed", installedText(rd, info))
}

// installedText lists the installed services of a device as text.
func installedText(rd radio, info *deviceInfo) string {
	services, err := installedServices(rd, info)
	if err != nil {
		return "error: " + err.Error()
	}
	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.String())
	}
	return strings.Join(names, ", ")
}

// newDevice converts what Windows reports and reads the services installed for
// the device. A failure to read the services is logged and leaves them empty, so
// the device still shows up in the list.
func (a *winAdapter) newDevice(rd radio, info *deviceInfo) Device {
	dev := Device{
		Name:          windows.UTF16ToString(info.name[:]),
		Address:       Address(info.address),
		Class:         ClassOfDevice(info.classOfDevice),
		Connected:     info.connected != 0,
		Remembered:    info.remembered != 0,
		Authenticated: info.authenticated != 0,
		LastSeen:      systemTime(info.lastSeen),
		LastUsed:      systemTime(info.lastUsed),
	}
	services, err := installedServices(rd, info)
	if err != nil {
		a.log.Warn("reading the installed services failed", "device", dev.DisplayName(), "address", dev.Address.String(), "err", err)
	}
	dev.Services = services
	return dev
}

// findDevice looks the paired device up on each radio.
func findDevice(radios []radio, addr Address) (radio, deviceInfo, error) {
	var lastErr error
	for _, rd := range radios {
		info, err := getDeviceInfo(rd, addr)
		if err != nil {
			if !errors.Is(err, windows.ERROR_NOT_FOUND) {
				lastErr = err
			}
			continue
		}
		if info.authenticated == 0 && info.remembered == 0 {
			continue
		}
		return rd, info, nil
	}
	if lastErr != nil {
		return radio{}, deviceInfo{}, fmt.Errorf("%w (%v)", ErrDeviceNotFound, lastErr)
	}
	return radio{}, deviceInfo{}, ErrDeviceNotFound
}
