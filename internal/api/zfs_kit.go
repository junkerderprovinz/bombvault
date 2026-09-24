package api

import (
	"fmt"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// zfsKitIntro is the part of the recovery kit's ZFS section that does not
// depend on what is configured. Every placeholder is single-quoted because a
// dataset name may contain spaces.
const zfsKitIntro = `
## ZFS datasets

Each ZFS item is a dataset together with the datasets below it. Every dataset
is backed up as its own ordinary restic snapshot whose root folder is that
dataset's root, tagged zfs:<dataset>. All datasets of one backup share the same
name at the end of their path (.zfs/snapshot/bombvault-<time>). Getting files
back needs restic only, no ZFS tools and no BombVault.
`

const zfsKitCommands = `
List the snapshots of one dataset:
    restic -r '<repo>' snapshots --tag 'zfs:<dataset>'

Restore the newest snapshot of one dataset into a new folder:
    restic -r '<repo>' restore latest --tag 'zfs:<dataset>' --target '/mnt/<pool>/restored-<name>'

After data loss the newest snapshot can be the emptied or encrypted one. The
list above shows each snapshot's size; use the id of an older one when the
newest is far smaller. If BombVault still runs, its Anomalies page names the
last good backup of each dataset.

Restore into the dataset itself (files with the same name are overwritten).
Take a ZFS snapshot first so you can go back:
    zfs snapshot '<dataset>@before-restore'
    restic -r '<repo>' restore '<snapshot-id>' --target '<mountpoint>'

Restore a single file or folder:
    restic -r '<repo>' restore '<snapshot-id>' --target '/tmp/restore' --include '/path/inside/the/dataset'

If a dataset does not exist any more, create it first with the properties you
want (for example: zfs create -o compression=lz4 '<dataset>') and restore into
its mountpoint. ZFS properties are not part of the backup.
`

// zfsKitSection is the recovery kit's ZFS chapter: the fixed explanation, one
// line per item with the repository its snapshots are in, and the restic
// commands that get the files back without BombVault.
func (s *Service) zfsKitSection(settings store.Settings) string {
	var b strings.Builder
	b.WriteString(zfsKitIntro)

	items, err := s.store.ListZFSDatasets()
	if err != nil {
		log.Printf("api: recovery kit: listing the ZFS items failed: %v", err)
	}
	if len(items) > 0 {
		b.WriteString("\nItems configured on this server:\n")
		for _, d := range items {
			fmt.Fprintf(&b, "- %s: repository %s", d.Dataset, s.zfsKitRepo(settings, d))
			if d.LastHostMountpoint != "" {
				fmt.Fprintf(&b, ", mounted at %s", d.LastHostMountpoint)
			}
			b.WriteString("\n")
			if detail := s.zfsKitItemDetail(d); detail != "" {
				fmt.Fprintf(&b, "  (%s)\n", detail)
			}
		}
	}

	b.WriteString(zfsKitCommands)
	return b.String()
}

// zfsKitRepo names where an item's snapshots are, by the name the user gave a
// named repository or by the resolved location of the domain's own.
func (s *Service) zfsKitRepo(settings store.Settings, d store.ZFSDataset) string {
	if id := strings.TrimSpace(d.Repo); id != "" {
		if n, err := s.store.GetNamedRepo(id); err == nil {
			if loc, rErr := s.resolveRepo(n.Repo); rErr == nil {
				return fmt.Sprintf("<named repository %q> %s", n.Name, loc)
			}
			return fmt.Sprintf("<named repository %q>", n.Name)
		}
	}
	loc, err := s.zfsRepoPath(settings)
	if err != nil {
		return "(not resolved)"
	}
	return loc
}

// zfsKitItemDetail lists what the item covered the last time it ran, so a
// reader knows which tags to look for and which datasets were never in it.
func (s *Service) zfsKitItemDetail(d store.ZFSDataset) string {
	var parts []string
	if members, err := s.store.ListZFSMembers(d.ID); err == nil && len(members) > 0 {
		names := make([]string, 0, len(members))
		for _, m := range members {
			names = append(names, m.Dataset)
		}
		parts = append(parts, "datasets: "+strings.Join(names, ", "))
	}
	if len(d.ExcludedChildren) > 0 {
		parts = append(parts, "left out: "+strings.Join(d.ExcludedChildren, ", "))
	}
	if len(d.StopContainers) > 0 {
		parts = append(parts, "stopped for the snapshot: "+strings.Join(d.StopContainers, ", "))
	}
	return strings.Join(parts, "; ")
}
