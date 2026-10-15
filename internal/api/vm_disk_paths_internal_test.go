package api

import (
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

func TestVMDiskContainerPathsAcceptsAnyKindOfDisk(t *testing.T) {
	const file = "<disk type='file' device='disk'><source file='/mnt/user/domains/vm/vdisk.qcow2'/><target dev='vda'/></disk>"
	const block = "<disk type='block' device='disk'><source dev='/dev/zvol/tank/vms/vm'/><target dev='vdb'/></disk>"
	s := &Service{cfg: config.Config{HostSourceRoot: "/mnt", HostMountRoot: "/host"}}

	tests := []struct {
		name    string
		disks   string
		want    []string
		wantErr string
	}{
		{name: "file disk only", disks: file, want: []string{"/host/user/domains/vm/vdisk.qcow2"}},
		{name: "zvol only", disks: block, want: []string{}},
		{name: "file and zvol", disks: file + block, want: []string{"/host/user/domains/vm/vdisk.qcow2"}},
		{name: "no disk", disks: "", wantErr: "no disk paths found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			domain, err := virshcli.ParseDomain("<domain type='kvm'><devices>" + tt.disks + "</devices></domain>")
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.vmDiskContainerPaths("vm", domain)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("paths = %v, want %v", got, tt.want)
			}
		})
	}
}
