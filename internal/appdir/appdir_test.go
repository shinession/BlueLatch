package appdir

import (
	"path/filepath"
	"testing"
)

func TestDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("LocalAppData", base)

	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	if want := filepath.Join(base, "BlueLatch"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

func TestDirWithoutLocalAppData(t *testing.T) {
	t.Setenv("LocalAppData", "")
	if got, err := Dir(); err == nil {
		t.Errorf("Dir() = %q, want an error when %%LocalAppData%% is not set", got)
	}
}
