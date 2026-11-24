package logging

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func logPath(dir string) string { return filepath.Join(dir, "BlueLatch", "bluelatch.log") }

func TestOpenWritesTheLogFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LocalAppData", dir)

	log, closeLog, err := Open(false, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	log.Debug("not wanted")
	log.Info("hello", "device", "Buds")
	closeLog()

	data, err := os.ReadFile(logPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	if text := string(data); !strings.Contains(text, "hello") || !strings.Contains(text, "device=Buds") {
		t.Errorf("log = %q, want the info line", text)
	} else if strings.Contains(text, "not wanted") {
		t.Errorf("log = %q, want no debug line without verbose", text)
	}
}

func TestOpenVerboseAlsoWritesDebugToStderr(t *testing.T) {
	t.Setenv("LocalAppData", t.TempDir())
	var stderr bytes.Buffer

	log, closeLog, err := Open(true, &stderr)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer closeLog()
	log.Debug("poll", "connected", false)

	if !strings.Contains(stderr.String(), "poll") {
		t.Errorf("stderr = %q, want the debug line", stderr.String())
	}
}

func TestOpenMovesALargeLogAside(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LocalAppData", dir)
	if err := os.MkdirAll(filepath.Dir(logPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath(dir), bytes.Repeat([]byte("x"), maxLogSize+1), 0o644); err != nil {
		t.Fatal(err)
	}

	log, closeLog, err := Open(false, nil)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	log.Info("fresh")
	closeLog()

	old, err := os.Stat(logPath(dir) + ".1")
	if err != nil || old.Size() != maxLogSize+1 {
		t.Errorf("the old log was not kept as bluelatch.log.1: %v", err)
	}
	if now, err := os.Stat(logPath(dir)); err != nil || now.Size() >= maxLogSize {
		t.Errorf("the new log is missing or still large: %v", err)
	}
}

func TestOpenWithoutAFileStillGivesALogger(t *testing.T) {
	t.Setenv("LocalAppData", "")
	var stderr bytes.Buffer

	log, closeLog, err := Open(true, &stderr)
	defer closeLog()
	if err == nil {
		t.Error("Open reported no error although %LocalAppData% is not set")
	}
	log.Info("still works")
	if !strings.Contains(stderr.String(), "still works") {
		t.Errorf("stderr = %q, want the line", stderr.String())
	}

	quiet, closeQuiet, _ := Open(false, nil)
	defer closeQuiet()
	quiet.Info("goes nowhere") // must not panic
}
