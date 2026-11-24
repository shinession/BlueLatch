package bt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// A journal is a folder of small files, one for every service switch that is in
// progress. Reconnect writes a file before it switches a service off and deletes
// it once the service has been switched on again. A file that is still there
// belongs to a switch that was interrupted, because the process died between the
// two calls, and the service may have been left switched off. Recover finds such
// files and switches the services on.
//
// One file per switch, instead of one shared list, keeps two processes (the
// window and a command line, say) from overwriting each other's records.
type journal struct{ dir string }

// pendingSwitch is the content of one file: Service of the device was switched
// off at Since, by the process PID, and has not been confirmed switched on.
type pendingSwitch struct {
	Device  string    `json:"device"`  // the name, for messages
	Address string    `json:"address"` // AA:BB:CC:DD:EE:FF
	Service string    `json:"service"` // the 16-bit ID in hex, such as 110B
	Since   time.Time `json:"since"`
	PID     int       `json:"pid"`

	path string // the file this was read from
}

const journalPrefix = "pending-"

// staleCorrupt is how old an unreadable file must be before it is removed, so a
// file another process is just writing is not taken for a corrupt one.
const staleCorrupt = time.Minute

func (j *journal) file(addr Address, s Service, pid int) string {
	return filepath.Join(j.dir, fmt.Sprintf("%s%012X-%04X-%d.json", journalPrefix, uint64(addr), uint16(s), pid))
}

// begin records that the service is about to be switched off.
func (j *journal) begin(device string, addr Address, s Service) error {
	rec := pendingSwitch{
		Device:  device,
		Address: addr.String(),
		Service: fmt.Sprintf("%04X", uint16(s)),
		Since:   time.Now(),
		PID:     os.Getpid(),
	}
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(j.dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(j.file(addr, s, rec.PID), data, 0o644)
}

// end records that the service has been switched on again.
func (j *journal) end(addr Address, s Service) error {
	err := os.Remove(j.file(addr, s, os.Getpid()))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// pending returns the records that are still there. A file that cannot be read
// is skipped and reported in the error; one that stays unreadable is removed
// after a while.
func (j *journal) pending() ([]pendingSwitch, error) {
	entries, err := os.ReadDir(j.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var found []pendingSwitch
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, journalPrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		path := filepath.Join(j.dir, name)
		rec, err := readPending(path)
		if err != nil {
			if info, statErr := e.Info(); statErr == nil && time.Since(info.ModTime()) > staleCorrupt {
				os.Remove(path)
			}
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		found = append(found, rec)
	}
	return found, errors.Join(errs...)
}

func readPending(path string) (pendingSwitch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pendingSwitch{}, err
	}
	var rec pendingSwitch
	if err := json.Unmarshal(data, &rec); err != nil {
		return pendingSwitch{}, err
	}
	if _, _, err := rec.parse(); err != nil {
		return pendingSwitch{}, err
	}
	rec.path = path
	return rec, nil
}

// parse returns the address and the service of the record.
func (p pendingSwitch) parse() (Address, Service, error) {
	addr, err := ParseAddress(p.Address)
	if err != nil {
		return 0, 0, err
	}
	n, err := strconv.ParseUint(p.Service, 16, 16)
	if err != nil {
		return 0, 0, fmt.Errorf("service %q: %w", p.Service, err)
	}
	return addr, Service(n), nil
}

// forget deletes the file of a record.
func (j *journal) forget(p pendingSwitch) error {
	err := os.Remove(p.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
