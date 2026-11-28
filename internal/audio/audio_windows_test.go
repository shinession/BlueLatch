//go:build windows

package audio

import (
	"os"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// A wrong digit in one of these would make Windows hand out another interface.
func TestTheIDs(t *testing.T) {
	for want, got := range map[string]windows.GUID{
		"{BCDE0395-E52F-467C-8E3D-C4579291692E}": clsidMMDeviceEnumerator,
		"{A95664D2-9614-4F35-A746-DE8DB63617E6}": iidIMMDeviceEnumerator,
		"{870AF99C-171D-4F9E-AF0D-E63DF40C2BC9}": clsidPolicyConfigClient,
		"{F8679F50-850A-41CF-9C72-430F290290C8}": iidIPolicyConfig,
		"{A45C254E-DF1C-4EFD-8020-67D146A850E0}": keyFriendlyName.fmtid,
		"{1DA5D803-D492-4EDD-8C23-E0C0FFEE7F0E}": keyFormFactor.fmtid,
	} {
		if !strings.EqualFold(got.String(), want) {
			t.Errorf("GUID = %s, want %s", got.String(), want)
		}
	}
}

func TestKindOfFormFactor(t *testing.T) {
	for factor, want := range map[uint32]Kind{
		1: KindSpeaker, 3: KindHeadphones, 5: KindHeadset, 9: KindDisplay,
		0: KindOther, 2: KindOther, 10: KindOther,
	} {
		if got := kindOfFormFactor(factor); got != want {
			t.Errorf("kindOfFormFactor(%d) = %q, want %q", factor, got, want)
		}
	}
}

func TestComErrorShowsTheHRESULT(t *testing.T) {
	err := &comError{"IPolicyConfig.SetDefaultEndpoint", 0x80070005}

	if got := err.Error(); !strings.Contains(got, "IPolicyConfig.SetDefaultEndpoint") || !strings.Contains(got, "0x80070005") {
		t.Errorf("Error() = %q, want the call and the HRESULT", got)
	}
}

// TestLiveOutputs reads the real outputs and checks that the object that changes
// the default can be created. It is read-only: it calls no method of that object.
// It only runs when BLUELATCH_LIVE=1.
func TestLiveOutputs(t *testing.T) {
	if os.Getenv("BLUELATCH_LIVE") != "1" {
		t.Skip("set BLUELATCH_LIVE=1 to read the real sound outputs")
	}
	a, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := a.State()
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range state.Outputs {
		t.Logf("%s kind=%s id=%s", o.Name, o.Kind, o.ID)
	}
	for role := RoleConsole; role <= RoleCommunications; role++ {
		id := state.Defaults[role]
		if id == "" {
			t.Logf("default %s: none", role)
			continue
		}
		o, ok := state.Output(id)
		if !ok {
			t.Errorf("the default of %s (%s) is not among the active outputs", role, id)
			continue
		}
		t.Logf("default %s: %s", role, o.Name)
	}

	err = withCOM(func() error {
		policy, err := create(&clsidPolicyConfigClient, &iidIPolicyConfig)
		if err == nil {
			policy.release()
		}
		return err
	})
	if err != nil {
		t.Errorf("the policy object cannot be created: %v", err)
	}
}
