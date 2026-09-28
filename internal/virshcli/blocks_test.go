package virshcli

import (
	"bytes"
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPullBackupXMLExportsOnlyBackedUpDisks(t *testing.T) {
	disks := []BackupDisk{
		{Dev: "vda", Scratch: "/mnt/user/domains/a & b/vda.bombvault-scratch"},
		{Dev: "sda", Skip: true},
	}
	got := PullBackupXML("/tmp/bombvault-nbd-x.sock", "bombvault-1", disks)
	for _, want := range []string{
		`<domainbackup mode="pull">`,
		"<incremental>bombvault-1</incremental>",
		`<server transport="unix" socket="/tmp/bombvault-nbd-x.sock"/>`,
		`<disk name="vda" backup="yes" type="file"><scratch file="/mnt/user/domains/a &amp; b/vda.bombvault-scratch"/></disk>`,
		`<disk name="sda" backup="no"/>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("backup xml lacks %s:\n%s", want, got)
		}
	}
	if strings.Contains(PullBackupXML("/s", "", disks), "incremental") {
		t.Error("a full backup names an incremental base")
	}
}

func TestCheckpointXMLTracksOnlyBackedUpDisks(t *testing.T) {
	got := CheckpointXML("bombvault-2", []BackupDisk{{Dev: "vda"}, {Dev: "hdc", Skip: true}})
	want := `<domaincheckpoint><name>bombvault-2</name><disks><disk name="vda" checkpoint="bitmap"/><disk name="hdc" checkpoint="no"/></disks></domaincheckpoint>`
	if got != want {
		t.Fatalf("got %s\nwant %s", got, want)
	}
}

func TestBackupJobSocketReadsTheServer(t *testing.T) {
	job := `<domainbackup mode='pull'>
  <incremental>bombvault-1</incremental>
  <server transport='unix' socket='/tmp/bombvault-nbd-x.sock'/>
  <disks><disk name='vda' backup='yes' type='file' exportname='vda' exportbitmap='backup-vda'/></disks>
</domainbackup>`
	if got := BackupJobSocket(job); got != "/tmp/bombvault-nbd-x.sock" {
		t.Fatalf("socket = %q", got)
	}
	if got := BackupJobSocket("<domainbackup mode='push'/>"); got != "" {
		t.Fatalf("push job socket = %q", got)
	}
}

func TestParseDomainKeepsDiskFormat(t *testing.T) {
	d, err := ParseDomain(`<domain><devices>
<disk type='file' device='disk'><driver name='qemu' type='qcow2'/><source file='/mnt/user/domains/x/vdisk1.qcow2'/><target dev='vda'/></disk>
<disk type='file' device='disk'><driver name='qemu' type='raw'/><source file='/mnt/user/domains/x/vdisk2.img'/><target dev='vdb'/></disk>
</devices></domain>`)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Disks) != 2 || d.Disks[0].Format != "qcow2" || d.Disks[1].Format != "raw" {
		t.Fatalf("disks = %+v", d.Disks)
	}
}

func TestNoBackupJobMatchesOnlyLibvirtsNoJobAnswer(t *testing.T) {
	if !noBackupJob("error: Domain backup job id not found: no domain backup job present\n") {
		t.Fatal("libvirt's no-job answer is not recognised")
	}
	for _, stderr := range []string{
		"error: failed to get domain 'win'\n",
		"error: Requested operation is not valid: domain is not running\n",
		"",
	} {
		if noBackupJob(stderr) {
			t.Fatalf("%q counts as no job", stderr)
		}
	}
}

// fakeVirsh writes a script that prints stderr and exits 1, and returns a
// client that runs it.
func fakeVirsh(t *testing.T, stderr string) *Client {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a shell script as the virsh binary")
	}
	dir := t.TempDir()
	msg := filepath.Join(dir, "stderr")
	if err := os.WriteFile(msg, []byte(stderr), 0o600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "virsh")
	script := "#!/bin/sh\ncat '" + msg + "' >&2\nexit 1\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the test needs it executable
		t.Fatal(err)
	}
	return &Client{bin: bin}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestBackupJobXMLTreatsNoJobAsNoneWithoutLogging(t *testing.T) {
	c := fakeVirsh(t, "error: Domain backup job id not found: no domain backup job present\n")
	logged := captureLog(t)
	job, err := c.BackupJobXML(context.Background(), "win")
	if err != nil || job != "" {
		t.Fatalf("job %q, err %v; want no job and no error", job, err)
	}
	if logged.Len() != 0 {
		t.Fatalf("logged %q for the normal case", logged.String())
	}
}

func TestBackupJobXMLReportsOtherFailures(t *testing.T) {
	c := fakeVirsh(t, "error: failed to get domain 'win'\n")
	logged := captureLog(t)
	if _, err := c.BackupJobXML(context.Background(), "win"); err == nil {
		t.Fatal("a missing domain came back without an error")
	}
	if !strings.Contains(logged.String(), "failed to get domain") {
		t.Fatalf("the failure was not logged: %q", logged.String())
	}
}

// recordingVirsh writes a script that appends its arguments, one per line,
// to a file and succeeds, and returns a client that runs it with that file.
func recordingVirsh(t *testing.T) (*Client, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a shell script as the virsh binary")
	}
	dir := t.TempDir()
	calls := filepath.Join(dir, "calls")
	bin := filepath.Join(dir, "virsh")
	script := "#!/bin/sh\nfor a in \"$@\"; do case \"$a\" in *.xml) cat \"$a\" >> '" + calls + "';; *) echo \"$a\" >> '" + calls + "';; esac; done\necho '--' >> '" + calls + "'\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil { //nolint:gosec // G306: the test needs it executable
		t.Fatal(err)
	}
	return &Client{bin: bin}, calls
}

func recordedCalls(t *testing.T, file string) []string {
	t.Helper()
	b, err := os.ReadFile(file) //nolint:gosec // G304: a file of the test
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, c := range strings.Split(strings.TrimSuffix(string(b), "--\n"), "\n--\n") {
		calls = append(calls, strings.Join(strings.Fields(c), " "))
	}
	return calls
}

func TestCheckpointForgetKeepsTheBitmap(t *testing.T) {
	c, file := recordingVirsh(t)
	if err := c.CheckpointForget(context.Background(), "win", "bombvault-1"); err != nil {
		t.Fatal(err)
	}
	if got := recordedCalls(t, file); len(got) != 1 || got[0] != "checkpoint-delete --metadata -- win bombvault-1" {
		t.Fatalf("calls = %q", got)
	}
}

func TestCheckpointRedefineSendsTheSavedDefinition(t *testing.T) {
	c, file := recordingVirsh(t)
	def := "<domaincheckpoint><name>bombvault-1</name></domaincheckpoint>"
	if err := c.CheckpointRedefine(context.Background(), "win", def, true); err != nil {
		t.Fatal(err)
	}
	if err := c.CheckpointRedefine(context.Background(), "win", def, false); err != nil {
		t.Fatal(err)
	}
	got := recordedCalls(t, file)
	want := []string{
		"checkpoint-create --redefine --redefine-validate -- win " + def,
		"checkpoint-create --redefine -- win " + def,
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("calls = %q, want %q", got, want)
	}
}
