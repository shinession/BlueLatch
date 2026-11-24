//go:build windows

package bt

import (
	"errors"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// recoverGraceAge is how long a record of a process that is still running is
// left alone: that process is probably switching the service right now. A
// switch never takes anywhere near this long, so an older record belongs to a
// process that hangs, or whose ID was reused.
const recoverGraceAge = 2 * time.Minute

// stillActive is the exit code GetExitCodeProcess reports for a running process.
const stillActive = 259

func (a *winAdapter) journalBegin(device string, addr Address, s Service) {
	if a.journal == nil {
		return
	}
	if err := a.journal.begin(device, addr, s); err != nil {
		a.log.Warn("the journal of service switches could not be written", "err", err)
	}
}

func (a *winAdapter) journalEnd(addr Address, s Service) {
	if a.journal == nil {
		return
	}
	if err := a.journal.end(addr, s); err != nil {
		a.log.Warn("the journal of service switches could not be updated", "err", err)
	}
}

// Recover implements Recoverer.
func (a *winAdapter) Recover() ([]Restored, error) {
	if a.journal == nil {
		return nil, nil
	}
	records, listErr := a.journal.pending()
	errs := []error{listErr}

	var todo []pendingSwitch
	for _, p := range records {
		if p.PID != os.Getpid() && processAlive(p.PID) && time.Since(p.Since) < recoverGraceAge {
			continue // another BlueLatch is switching this service right now
		}
		todo = append(todo, p)
	}
	if len(todo) == 0 {
		return nil, errors.Join(errs...)
	}

	radios, err := openRadios()
	if err != nil {
		// The records stay, and the next start tries again.
		return nil, errors.Join(append(errs, err)...)
	}
	defer closeRadios(radios)

	var restored []Restored
	for _, p := range todo {
		addr, svc, err := p.parse()
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rd, info, err := findDevice(radios, addr)
		if errors.Is(err, ErrDeviceNotFound) {
			a.log.Info("the device of a pending service switch is not paired any more", "device", p.Device, "address", p.Address)
			errs = append(errs, a.journal.forget(p))
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if err := setServiceState(rd, &info, svc, serviceEnable); err != nil {
			errs = append(errs, err)
			continue
		}
		a.log.Warn("switched on a service that an earlier run had left switched off", "device", p.Device, "address", p.Address, "service", svc.String())
		errs = append(errs, a.journal.forget(p))
		restored = append(restored, Restored{Device: p.Device, Address: addr, Service: svc})
	}
	return restored, errors.Join(errs...)
}

// processAlive reports whether a process with this ID is running.
func processAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// ERROR_INVALID_PARAMETER means there is no such process. Access denied
		// means there is one that this user may not look at.
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}
