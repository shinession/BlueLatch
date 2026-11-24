package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestADetachedProcessNeverStartsAnother(t *testing.T) {
	t.Setenv(detachedEnv, "1")
	if got := (winConsole{}).State(); got != consoleNone {
		t.Errorf("State() = %d with %s set, want %d (none)", got, detachedEnv, consoleNone)
	}
}

// markerEnv names the file in which the helper process of TestStartDetached
// leaves its report.
const markerEnv = "BLUELATCH_TEST_MARKER"

// TestDetachedHelper is not a test of its own: it is the process that
// TestStartDetached starts, and it only reports how it was started.
func TestDetachedHelper(t *testing.T) {
	marker := os.Getenv(markerEnv)
	if marker == "" {
		t.Skip("runs only as the helper of TestStartDetached")
	}
	report := fmt.Sprintf("processes=%d state=%d guard=%s", consoleProcesses(), (winConsole{}).State(), os.Getenv(detachedEnv))
	if err := os.WriteFile(marker, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStartDetached(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "report.txt")
	t.Setenv(markerEnv, marker)

	// The test binary stands in for the program: it is started again, and has to
	// find itself without a console and with the guard set.
	if err := (winConsole{}).StartDetached([]string{"-test.run=^TestDetachedHelper$"}); err != nil {
		t.Fatal(err)
	}
	const want = "processes=0 state=0 guard=1"
	var got string
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(25 * time.Millisecond) {
		if b, err := os.ReadFile(marker); err == nil {
			if got = string(b); got == want {
				return
			}
		}
	}
	t.Errorf("the started process reported %q, want %q", got, want)
}
