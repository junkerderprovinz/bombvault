package api

import (
	"context"
	"slices"
	"testing"
)

// stateVirsh answers each VM's state from a map and records the starts.
type stateVirsh struct {
	scriptedVirsh
	states  map[string]string
	started []string
}

func (v *stateVirsh) State(_ context.Context, name string) (string, error) {
	return v.states[name], nil
}

func (v *stateVirsh) Start(_ context.Context, name string) error {
	v.started = append(v.started, name)
	return nil
}

// A BombVault stopped in the middle of a graceful backup leaves a router or
// Home Assistant VM shut down. At its next start the VM is started again; one
// that runs or is gone is forgotten, one still shutting down is kept.
func TestRecoverVMRestartsStartsTheVMsABackupLeftDown(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	v := &stateVirsh{states: map[string]string{"pfsense": "shut off", "homeassistant": "running", "windows": "in shutdown"}}
	svc.virsh = v
	for _, name := range []string{"pfsense", "homeassistant", "windows", "deleted"} {
		if err := st.MarkVMStopped(name, 1); err != nil {
			t.Fatal(err)
		}
	}

	svc.RecoverVMRestarts(context.Background())

	if !slices.Equal(v.started, []string{"pfsense"}) {
		t.Fatalf("started = %v, want only the VM that is shut off", v.started)
	}
	left, err := st.ListVMsStopped()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(left, []string{"windows"}) {
		t.Fatalf("still recorded = %v, want the VM that is still shutting down", left)
	}
}
