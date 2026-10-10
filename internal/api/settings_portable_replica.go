package api

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
)

// zfsReplicaExport is the replica part of a settings file: the ZFS servers
// and how each ZFS item replicates, keyed by its root dataset because item
// ids differ between instances. Keys stay on the instance, and neither the
// receive slots a person allowed here nor a peer target's answer travels: a
// peer target asks again from the instance the file is applied to.
type zfsReplicaExport struct {
	Servers []zfsReplicaServerExport `json:"servers"`
	Items   []zfsReplicaItemExport   `json:"items"`
}

type zfsReplicaServerExport struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Host    string `json:"host"`
	User    string `json:"user"`
	Port    int    `json:"port"`
	Pool    string `json:"pool"`
	Root    string `json:"root"`
	Enabled bool   `json:"enabled"`
}

type zfsReplicaTargetExport struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type zfsReplicaItemExport struct {
	Dataset     string                 `json:"dataset"`
	Target      zfsReplicaTargetExport `json:"target"`
	AfterBackup bool                   `json:"afterBackup"`
	Cadence     string                 `json:"cadence"`
	Keep        store.ZFSReplicaKeep   `json:"keep"`
}

// exportZFSReplica adds the ZFS servers and every item's replica settings to
// an export.
func (h *Handler) exportZFSReplica(exp *settingsExport) error {
	servers, err := h.store.ListZFSReplicaServers()
	if err != nil {
		return err
	}
	items, err := h.store.ListZFSDatasets()
	if err != nil {
		return err
	}
	out := &zfsReplicaExport{Servers: []zfsReplicaServerExport{}, Items: []zfsReplicaItemExport{}}
	for _, s := range servers {
		out.Servers = append(out.Servers, zfsReplicaServerExport{
			ID: s.ID, Name: s.Name, Host: s.Host, User: s.User, Port: s.Port, Pool: s.Pool, Root: s.Root, Enabled: s.Enabled,
		})
	}
	for _, d := range items {
		rep := d.Replica
		out.Items = append(out.Items, zfsReplicaItemExport{
			Dataset:     d.Dataset,
			Target:      zfsReplicaTargetExport{Kind: rep.TargetKind, ID: rep.TargetID},
			AfterBackup: rep.AfterBackup,
			Cadence:     rep.Cadence,
			Keep:        rep.Keep,
		})
	}
	exp.ZFSReplica = out
	return nil
}

// zfsReplicaServerRefusal checks a server the way a form has to: everything
// but the name ends up in an ssh or zfs argv, so a host or user that starts
// with a dash or holds a space is refused.
func zfsReplicaServerRefusal(name, host, user string, port int, pool, root string) string {
	switch {
	case strings.TrimSpace(name) == "":
		return "needs a name"
	case !argvSafe(host):
		return "needs a host without spaces that does not start with a dash"
	case !argvSafe(user):
		return "needs a user without spaces that does not start with a dash"
	case port < 1 || port > 65535:
		return fmt.Sprintf("port %d is not between 1 and 65535", port)
	}
	if err := zfs.ValidateDatasetName(pool); err != nil || strings.Contains(pool, "/") {
		return fmt.Sprintf("pool %q is not a pool name", pool)
	}
	if err := zfs.ValidateDatasetName(root); err != nil {
		return fmt.Sprintf("root %q: %v", root, err)
	}
	if root != pool && !strings.HasPrefix(root, pool+"/") {
		return fmt.Sprintf("root %q is not on pool %q", root, pool)
	}
	return ""
}

func argvSafe(s string) bool {
	return s != "" && s[0] != '-' && !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	})
}

// zfsReplicaItemRefusal checks one item's replica settings. servers holds
// the ids a server target may name.
func zfsReplicaItemRefusal(target zfsReplicaTargetExport, cadence string, keep store.ZFSReplicaKeep, servers map[string]bool) string {
	switch target.Kind {
	case store.ZFSReplicaTargetNone:
		if target.ID != "" {
			return "a target of kind none names " + target.ID
		}
	case store.ZFSReplicaTargetServer:
		if !servers[target.ID] {
			return fmt.Sprintf("replicates to server %q, which does not exist", target.ID)
		}
	case store.ZFSReplicaTargetPeer:
		if strings.TrimSpace(target.ID) == "" {
			return "a peer target names no instance"
		}
	default:
		return fmt.Sprintf("unknown target kind %q", target.Kind)
	}
	if _, ok := keep.Counts(); !ok {
		return fmt.Sprintf("unknown keep preset %q", keep.Preset)
	}
	for _, n := range keep.Own {
		if n < 0 {
			return "a keep count is negative"
		}
	}
	// The scheduler gives an item no entry of its own for everyN, so such a
	// replica would never run.
	if err := zfsValidateCadence(cadence); err != nil {
		return "cadence: " + scrubError(err)
	}
	return ""
}

// zfsReplicaRefusal checks the replica block of a file before anything is
// written.
func zfsReplicaRefusal(exp settingsExport) string {
	rep := exp.ZFSReplica
	if rep == nil {
		return ""
	}
	ids := map[string]bool{}
	for i, s := range rep.Servers {
		// The id names the server's known_hosts directory.
		if !validResourceName(s.ID) || ids[s.ID] {
			return fmt.Sprintf("ZFS server #%d: needs an id of its own", i+1)
		}
		ids[s.ID] = true
		if msg := zfsReplicaServerRefusal(s.Name, s.Host, s.User, s.Port, s.Pool, s.Root); msg != "" {
			return fmt.Sprintf("ZFS server #%d (%s): %s", i+1, s.Name, msg)
		}
	}
	seen := map[string]bool{}
	for i, it := range rep.Items {
		if strings.TrimSpace(it.Dataset) == "" || seen[it.Dataset] {
			return fmt.Sprintf("ZFS replica #%d: needs a dataset of its own", i+1)
		}
		seen[it.Dataset] = true
		if msg := zfsReplicaItemRefusal(it.Target, it.Cadence, it.Keep, ids); msg != "" {
			return fmt.Sprintf("ZFS replica of %s: %s", it.Dataset, msg)
		}
	}
	return ""
}

// zfsReplicaGroups names the replica in the preview when the file sets up a
// server or a target.
func zfsReplicaGroups(exp settingsExport) []string {
	rep := exp.ZFSReplica
	if rep == nil {
		return nil
	}
	if len(rep.Servers) > 0 {
		return []string{"zfsReplica"}
	}
	for _, it := range rep.Items {
		if it.Target.Kind != store.ZFSReplicaTargetNone {
			return []string{"zfsReplica"}
		}
	}
	return nil
}

// applyImportedZFSReplica writes the replica block of a file. A file from a
// build without it leaves this instance's servers and settings alone.
func (h *Handler) applyImportedZFSReplica(exp settingsExport) error {
	rep := exp.ZFSReplica
	if rep == nil {
		return nil
	}
	in := store.ZFSReplicaImport{Items: map[string]store.ZFSReplica{}}
	for _, s := range rep.Servers {
		in.Servers = append(in.Servers, store.ZFSReplicaServer{
			ID: s.ID, Name: strings.TrimSpace(s.Name), Host: s.Host, User: s.User, Port: s.Port,
			Pool: s.Pool, Root: s.Root, Enabled: s.Enabled,
		})
	}
	for _, it := range rep.Items {
		in.Items[it.Dataset] = store.ZFSReplica{
			TargetKind: it.Target.Kind, TargetID: it.Target.ID,
			AfterBackup: it.AfterBackup, Cadence: it.Cadence, Keep: it.Keep,
		}
	}
	return h.svc.importZFSReplica(in)
}

// importZFSReplica writes an imported replica block the way the forms would:
// every item it gives another target, or whose server it drops or moves, has
// its replica lock taken first, and leaving a target cleans the item's tree.
func (s *Service) importZFSReplica(in store.ZFSReplicaImport) error {
	items, err := s.store.ListZFSDatasets()
	if err != nil {
		return err
	}
	servers, err := s.store.ListZFSReplicaServers()
	if err != nil {
		return err
	}
	incoming := make(map[string]store.ZFSReplicaServer, len(in.Servers))
	for _, srv := range in.Servers {
		incoming[srv.ID] = srv
	}
	elsewhere := map[string]bool{}
	for _, srv := range servers {
		next, kept := incoming[srv.ID]
		elsewhere[srv.ID] = !kept || store.ZFSReplicaServerMoved(srv, next)
	}
	var touched []string
	var leaving []store.ZFSDataset
	for _, d := range items {
		cur := d.Replica
		next, inFile := in.Items[d.Dataset]
		retarget := inFile && (next.TargetKind != cur.TargetKind || next.TargetID != cur.TargetID)
		onServer := cur.TargetKind == store.ZFSReplicaTargetServer
		_, kept := incoming[cur.TargetID]
		if retarget || (onServer && elsewhere[cur.TargetID]) {
			touched = append(touched, d.ID)
		}
		if cur.TargetKind != store.ZFSReplicaTargetNone && (retarget || (onServer && !kept)) {
			leaving = append(leaving, d)
		}
	}
	unlock, ok := s.lockZFSReplicas(touched)
	if !ok {
		return errZFSReplicaBusy
	}
	defer unlock()
	for _, d := range leaving {
		s.cleanZFSReplicaSource(context.Background(), d)
	}
	return s.store.ImportZFSReplica(in)
}
