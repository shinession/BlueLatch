package bt

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestJournal(t *testing.T) *journal {
	t.Helper()
	return &journal{dir: filepath.Join(t.TempDir(), "pending")}
}

func TestJournalRecordsAndForgetsASwitch(t *testing.T) {
	j := newTestJournal(t)

	if err := j.begin("Example Buds", 0xAABBCCDDEE02, ServiceA2DPSink); err != nil {
		t.Fatalf("begin: %v", err)
	}
	got, err := j.pending()
	if err != nil || len(got) != 1 {
		t.Fatalf("pending = %v, %v; want one record", got, err)
	}
	p := got[0]
	addr, svc, err := p.parse()
	if err != nil || addr != 0xAABBCCDDEE02 || svc != ServiceA2DPSink {
		t.Errorf("record = %v %v, %v; want the address and the A2DP sink", addr, svc, err)
	}
	if p.Device != "Example Buds" || p.PID != os.Getpid() || time.Since(p.Since) > time.Minute {
		t.Errorf("record = %+v; want the device, this process and a recent time", p)
	}

	if err := j.end(0xAABBCCDDEE02, ServiceA2DPSink); err != nil {
		t.Fatalf("end: %v", err)
	}
	if got, err := j.pending(); err != nil || len(got) != 0 {
		t.Errorf("pending after end = %v, %v; want none", got, err)
	}
	if err := j.end(0xAABBCCDDEE02, ServiceA2DPSink); err != nil {
		t.Errorf("a second end: %v; want it to be harmless", err)
	}
}

func TestJournalKeepsRecordsOfDifferentServicesApart(t *testing.T) {
	j := newTestJournal(t)
	for _, s := range []Service{ServiceA2DPSink, ServiceHandsfree} {
		if err := j.begin("Example Buds", 0xAABBCCDDEE02, s); err != nil {
			t.Fatalf("begin %v: %v", s, err)
		}
	}
	if err := j.end(0xAABBCCDDEE02, ServiceA2DPSink); err != nil {
		t.Fatal(err)
	}
	got, err := j.pending()
	if err != nil || len(got) != 1 {
		t.Fatalf("pending = %v, %v; want only the Hands-Free record", got, err)
	}
	if _, svc, _ := got[0].parse(); svc != ServiceHandsfree {
		t.Errorf("left record is for %v, want Hands-Free", svc)
	}
	if err := j.forget(got[0]); err != nil {
		t.Fatalf("forget: %v", err)
	}
	if got, _ := j.pending(); len(got) != 0 {
		t.Errorf("pending after forget = %v, want none", got)
	}
}

func TestJournalWithoutAFolder(t *testing.T) {
	j := &journal{dir: filepath.Join(t.TempDir(), "never created")}
	if got, err := j.pending(); err != nil || got != nil {
		t.Errorf("pending = %v, %v; want nothing and no error", got, err)
	}
}

func TestJournalHandlesUnreadableFiles(t *testing.T) {
	j := newTestJournal(t)
	if err := os.MkdirAll(j.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fresh := filepath.Join(j.dir, journalPrefix+"fresh.json")
	stale := filepath.Join(j.dir, journalPrefix+"stale.json")
	for _, p := range []string{fresh, stale} {
		if err := os.WriteFile(p, []byte("{not json"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	if err := j.begin("Example Buds", 0xAABBCCDDEE02, ServiceA2DPSink); err != nil {
		t.Fatal(err)
	}

	got, err := j.pending()

	if len(got) != 1 {
		t.Errorf("pending = %v; want the good record only", got)
	}
	if err == nil {
		t.Error("pending reported no error for the unreadable files")
	}
	if _, statErr := os.Stat(fresh); statErr != nil {
		t.Error("a fresh unreadable file was removed; another process may be writing it")
	}
	if _, statErr := os.Stat(stale); statErr == nil {
		t.Error("a stale unreadable file was kept")
	}
}

func TestJournalIgnoresOtherFiles(t *testing.T) {
	j := newTestJournal(t)
	if err := os.MkdirAll(j.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(j.dir, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := j.pending(); err != nil || len(got) != 0 {
		t.Errorf("pending = %v, %v; want none", got, err)
	}
}
