package virshcli

import (
	"strings"
	"testing"
)

func TestEnsureNVRAMTemplate(t *testing.T) {
	const loader = `<loader readonly='yes' type='pflash'>/usr/share/qemu/ovmf-x64/OVMF_CODE-pure-efi.fd</loader>`
	const nvramPath = `/etc/libvirt/qemu/nvram/abc_VARS-pure-efi.fd`

	t.Run("injects template derived from loader", func(t *testing.T) {
		in := `<domain><os>` + loader + `<nvram>` + nvramPath + `</nvram></os></domain>`
		got := EnsureNVRAMTemplate(in)
		want := `<nvram template='/usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd'>` + nvramPath + `</nvram>`
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	})

	t.Run("BIOS domain (no nvram) is unchanged", func(t *testing.T) {
		in := `<domain><os><type>hvm</type><boot dev='hd'/></os></domain>`
		if got := EnsureNVRAMTemplate(in); got != in {
			t.Fatalf("expected unchanged, got:\n%s", got)
		}
	})

	t.Run("existing template is preserved", func(t *testing.T) {
		in := `<domain><os>` + loader +
			`<nvram template='/custom/MASTER.fd'>` + nvramPath + `</nvram></os></domain>`
		if got := EnsureNVRAMTemplate(in); got != in {
			t.Fatalf("expected unchanged (already has template), got:\n%s", got)
		}
	})

	t.Run("no loader falls back to stock OVMF master", func(t *testing.T) {
		in := `<domain><os><nvram>` + nvramPath + `</nvram></os></domain>`
		got := EnsureNVRAMTemplate(in)
		want := `<nvram template='` + defaultOVMFVars + `'>`
		if !strings.Contains(got, want) {
			t.Fatalf("expected fallback %q in output, got:\n%s", want, got)
		}
	})

	t.Run("nvram attributes are preserved", func(t *testing.T) {
		in := `<domain><os>` + loader + `<nvram type='file'>` + nvramPath + `</nvram></os></domain>`
		got := EnsureNVRAMTemplate(in)
		want := `<nvram template='/usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd' type='file'>`
		if !strings.Contains(got, want) {
			t.Fatalf("expected %q in output, got:\n%s", want, got)
		}
	})
}

func TestDeriveVarsTemplate(t *testing.T) {
	cases := []struct{ in, want string }{
		{`<loader>/usr/share/qemu/ovmf-x64/OVMF_CODE-pure-efi.fd</loader>`, "/usr/share/qemu/ovmf-x64/OVMF_VARS-pure-efi.fd"},
		{`<loader>/usr/share/OVMF/OVMF_CODE.secboot.fd</loader>`, "/usr/share/OVMF/OVMF_VARS.secboot.fd"},
		{`<domain></domain>`, defaultOVMFVars},                   // no loader
		{`<loader>/weird/firmware.fd</loader>`, defaultOVMFVars}, // no CODE marker
	}
	for _, tc := range cases {
		if got := deriveVarsTemplate(tc.in); got != tc.want {
			t.Errorf("deriveVarsTemplate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
