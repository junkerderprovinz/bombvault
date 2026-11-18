package virshcli

import (
	"context"
	"strings"
)

// DomainNames lists every domain, running or not, by the name virsh knows it
// by.
func (c *Client) DomainNames(ctx context.Context) ([]string, error) {
	out, err := c.run(ctx, "list", "--all", "--name")
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

// SnapshotTable returns virsh's table of the snapshots libvirt records for a
// domain, with their creation times.
func (c *Client) SnapshotTable(ctx context.Context, domain string) (string, error) {
	return c.run(ctx, "snapshot-list", "--domain", domain)
}

// SnapshotCreate takes a snapshot libvirt keeps a record of, in the disk
// images themselves where their format allows it.
func (c *Client) SnapshotCreate(ctx context.Context, domain, name string) error {
	_, err := c.run(ctx, "snapshot-create-as", "--domain", domain, "--name", name, "--atomic")
	return err
}

// SnapshotDelete removes a snapshot and libvirt's record of it.
func (c *Client) SnapshotDelete(ctx context.Context, domain, name string) error {
	_, err := c.run(ctx, "snapshot-delete", "--domain", domain, "--snapshotname", name)
	return err
}
