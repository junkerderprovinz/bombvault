package virshcli

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"strings"
)

// BlockBackups is the libvirt backup API that a changed-block VM backup
// needs: checkpoints, pull-mode backup jobs and guest freeze. It is separate
// from Virsh so the many test doubles of Virsh need not grow; a Virsh that
// does not implement it simply never takes the changed-block path.
type BlockBackups interface {
	// CheckpointNames lists the domain's checkpoints.
	CheckpointNames(ctx context.Context, domain string) ([]string, error)
	// CheckpointDelete removes one checkpoint and merges its bitmap away.
	// A checkpoint that is already gone is not an error.
	CheckpointDelete(ctx context.Context, domain, checkpoint string) error
	// CheckpointForget removes only libvirt's record of a checkpoint and
	// leaves its bitmap in the disk image. A checkpoint that is already gone
	// is not an error.
	CheckpointForget(ctx context.Context, domain, checkpoint string) error
	// CheckpointXML returns a checkpoint's definition without the domain.
	CheckpointXML(ctx context.Context, domain, checkpoint string) (string, error)
	// CheckpointRedefine brings back a forgotten checkpoint from its
	// definition. With validate, libvirt first checks that the bitmap is
	// still in the image, which it can only do while the domain runs.
	CheckpointRedefine(ctx context.Context, domain, checkpointXML string, validate bool) error
	// BackupBegin starts a backup job from a <domainbackup> document and
	// creates the checkpoint described by checkpointXML at the same instant.
	BackupBegin(ctx context.Context, domain, backupXML, checkpointXML string) error
	// BackupJobXML returns the running backup job's description, or "" when
	// the domain runs none.
	BackupJobXML(ctx context.Context, domain string) (string, error)
	// AbortJob ends the domain's running job. No job is not an error.
	AbortJob(ctx context.Context, domain string) error
	// FSFreeze and FSThaw freeze and thaw the guest's filesystems through
	// the guest agent.
	FSFreeze(ctx context.Context, domain string) error
	FSThaw(ctx context.Context, domain string) error
}

var _ BlockBackups = (*Client)(nil)

// CheckpointNames implements BlockBackups.
func (c *Client) CheckpointNames(ctx context.Context, domain string) ([]string, error) {
	out, err := c.run(ctx, "checkpoint-list", domain, "--name")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if n := strings.TrimSpace(line); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// CheckpointDelete implements BlockBackups.
func (c *Client) CheckpointDelete(ctx context.Context, domain, checkpoint string) error {
	_, err := c.run(ctx, "checkpoint-delete", domain, checkpoint)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "no domain checkpoint") {
		return nil
	}
	return err
}

// CheckpointForget implements BlockBackups.
func (c *Client) CheckpointForget(ctx context.Context, domain, checkpoint string) error {
	_, err := c.run(ctx, "checkpoint-delete", "--metadata", "--", domain, checkpoint)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "no domain checkpoint") {
		return nil
	}
	return err
}

// CheckpointXML implements BlockBackups.
func (c *Client) CheckpointXML(ctx context.Context, domain, checkpoint string) (string, error) {
	return c.run(ctx, "checkpoint-dumpxml", "--no-domain", "--", domain, checkpoint)
}

// CheckpointRedefine implements BlockBackups.
func (c *Client) CheckpointRedefine(ctx context.Context, domain, checkpointXML string, validate bool) error {
	f, err := writeTemp("bombvault-checkpoint-*.xml", checkpointXML)
	if err != nil {
		return err
	}
	defer os.Remove(f) //nolint:errcheck // a leftover temp file is harmless
	args := []string{"checkpoint-create", "--redefine"}
	if validate {
		args = append(args, "--redefine-validate")
	}
	_, err = c.run(ctx, append(args, "--", domain, f)...)
	return err
}

// BackupBegin implements BlockBackups. virsh reads both documents on the
// client side, so they go to local temporary files.
func (c *Client) BackupBegin(ctx context.Context, domain, backupXML, checkpointXML string) error {
	bk, err := writeTemp("bombvault-backup-*.xml", backupXML)
	if err != nil {
		return err
	}
	defer os.Remove(bk) //nolint:errcheck // a leftover temp file is harmless
	cp, err := writeTemp("bombvault-checkpoint-*.xml", checkpointXML)
	if err != nil {
		return err
	}
	defer os.Remove(cp) //nolint:errcheck // a leftover temp file is harmless
	_, err = c.run(ctx, "backup-begin", domain, bk, cp)
	return err
}

func writeTemp(pattern, content string) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("virshcli: temp file: %w", err)
	}
	_, werr := f.WriteString(content)
	cerr := f.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("virshcli: write temp file: %v %v", werr, cerr)
	}
	return f.Name(), nil
}

// BackupJobXML implements BlockBackups. No running job is the usual answer,
// so it is neither an error nor logged.
func (c *Client) BackupJobXML(ctx context.Context, domain string) (string, error) {
	out, stderr, err := c.exec(ctx, "backup-dumpxml", domain)
	if err != nil {
		if noBackupJob(stderr) {
			return "", nil
		}
		log.Printf("virshcli: %q failed: %s", "backup-dumpxml", stderr)
		return "", fmt.Errorf("virshcli: backup-dumpxml: %s", lastReason(stderr))
	}
	return out, nil
}

// noBackupJob reports whether virsh stderr is libvirt's answer for a domain
// that runs no backup job: "Domain backup job id not found: no domain backup
// job present".
func noBackupJob(stderr string) bool {
	return strings.Contains(stderr, "no domain backup job present")
}

// AbortJob implements BlockBackups.
func (c *Client) AbortJob(ctx context.Context, domain string) error {
	_, err := c.run(ctx, "domjobabort", domain)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "no job") {
		return nil
	}
	return err
}

// FSFreeze implements BlockBackups.
func (c *Client) FSFreeze(ctx context.Context, domain string) error {
	_, err := c.run(ctx, "domfsfreeze", domain)
	return err
}

// FSThaw implements BlockBackups.
func (c *Client) FSThaw(ctx context.Context, domain string) error {
	_, err := c.run(ctx, "domfsthaw", domain)
	return err
}

// BackupDisk is one disk of a backup job. A disk with Skip set is left out of
// both the backup and the checkpoint; Scratch is where qemu keeps the old
// data of blocks the guest writes while the job runs.
type BackupDisk struct {
	Dev     string
	Scratch string
	Skip    bool
}

// PullBackupXML is the <domainbackup> document for a pull-mode job that
// exports the disks on the Unix socket. A non-empty incremental makes the
// export carry a dirty bitmap of the changes since that checkpoint.
func PullBackupXML(socket, incremental string, disks []BackupDisk) string {
	var b strings.Builder
	b.WriteString(`<domainbackup mode="pull">`)
	if incremental != "" {
		b.WriteString("<incremental>" + xmlText(incremental) + "</incremental>")
	}
	b.WriteString(`<server transport="unix" socket="` + xmlText(socket) + `"/><disks>`)
	for _, d := range disks {
		if d.Skip {
			b.WriteString(`<disk name="` + xmlText(d.Dev) + `" backup="no"/>`)
			continue
		}
		b.WriteString(`<disk name="` + xmlText(d.Dev) + `" backup="yes" type="file"><scratch file="` + xmlText(d.Scratch) + `"/></disk>`)
	}
	b.WriteString("</disks></domainbackup>")
	return b.String()
}

// CheckpointXML is the <domaincheckpoint> document that tracks the disks not
// skipped in a bitmap from now on.
func CheckpointXML(name string, disks []BackupDisk) string {
	var b strings.Builder
	b.WriteString("<domaincheckpoint><name>" + xmlText(name) + "</name><disks>")
	for _, d := range disks {
		mode := "bitmap"
		if d.Skip {
			mode = "no"
		}
		b.WriteString(`<disk name="` + xmlText(d.Dev) + `" checkpoint="` + mode + `"/>`)
	}
	b.WriteString("</disks></domaincheckpoint>")
	return b.String()
}

func xmlText(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// BackupJobSocket is the Unix socket a running pull-mode backup job exports
// on, read from BackupJobXML's document; "" when it names none.
func BackupJobSocket(jobXML string) string {
	var doc struct {
		Server struct {
			Socket string `xml:"socket,attr"`
		} `xml:"server"`
	}
	if xml.Unmarshal([]byte(jobXML), &doc) != nil {
		return ""
	}
	return doc.Server.Socket
}
