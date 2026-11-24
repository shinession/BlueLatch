//go:build windows

package ui

import (
	"strings"
	"testing"
)

func TestThePageIsEmbeddedAndTalksToTheBoundFunctions(t *testing.T) {
	for _, want := range []string{"<title>BlueLatch</title>", "blState", "blConnect", "blCancel", "Content-Security-Policy"} {
		if !strings.Contains(indexHTML, want) {
			t.Errorf("the embedded page lacks %q", want)
		}
	}
}

// The page sets the names of devices as text. A name is chosen by the device, so
// it must never reach innerHTML, which is used only for the fixed icons.
func TestThePageNeverPutsDeviceTextIntoInnerHTML(t *testing.T) {
	if got := strings.Count(indexHTML, ".innerHTML"); got != 1 {
		t.Errorf("the page uses innerHTML %d times, want exactly once (for the icons)", got)
	}
	if !strings.Contains(indexHTML, "c.icon.innerHTML = ICONS[") {
		t.Error("the one use of innerHTML is not the icon assignment")
	}
}

func TestThePageLoadsNothingFromOutside(t *testing.T) {
	for _, bad := range []string{"http://", "https://", "<link ", "src=\"http"} {
		if strings.Contains(indexHTML, bad) {
			t.Errorf("the page contains %q, but it must load nothing from outside", bad)
		}
	}
}

func TestSingleInstance(t *testing.T) {
	name := `Local\BlueLatch-Test-` + t.Name()
	release, ok := SingleInstance(name, "no such window")
	if !ok {
		t.Fatal("the first SingleInstance reported another instance")
	}
	defer release()
	if _, ok := SingleInstance(name, "no such window"); ok {
		t.Error("the second SingleInstance did not notice the first")
	}
}
