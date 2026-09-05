package dynsec

import "testing"

func TestMasterRoleName(t *testing.T) {
	const masterID = "master-001"
	if got, want := masterRoleName(masterID), "host-master-telemetry-master-001"; got != want {
		t.Fatalf("masterRoleName(%q) = %q, want %q", masterID, got, want)
	}
}
