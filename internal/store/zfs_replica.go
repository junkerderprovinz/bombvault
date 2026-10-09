package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"
)

// Where a ZFS item replicates to.
const (
	ZFSReplicaTargetNone   = "none"
	ZFSReplicaTargetServer = "server"
	ZFSReplicaTargetPeer   = "peer"
)

// ZFSReplicaRunKind is the runs.kind of a replica run.
const ZFSReplicaRunKind = "replica"

// ZFSReplicaKeep is what an item keeps on its target: a preset, or with
// preset "own" the counts in Own as latest, daily, weekly, monthly, yearly.
type ZFSReplicaKeep struct {
	Preset string `json:"preset"`
	Own    [5]int `json:"own"`
}

var zfsReplicaKeepPresets = map[string][5]int{
	"short":    {0, 7, 4, 3, 0},
	"balanced": {0, 7, 4, 6, 1},
	"long":     {0, 14, 8, 12, 3},
}

// DefaultZFSReplicaKeep is about a month: 7 daily and 3 weekly snapshots. The
// column defaults in the migrations spell out the same rule.
var DefaultZFSReplicaKeep = ZFSReplicaKeep{Preset: "own", Own: [5]int{0, 7, 3, 0, 0}}

// Counts returns the keep counts the rule stands for, and false for a preset
// this build does not know.
func (k ZFSReplicaKeep) Counts() (RetentionKeep, bool) {
	n := k.Own
	if k.Preset != "own" {
		p, ok := zfsReplicaKeepPresets[k.Preset]
		if !ok {
			return RetentionKeep{}, false
		}
		n = p
	}
	return RetentionKeep{KeepLast: n[0], KeepDaily: n[1], KeepWeekly: n[2], KeepMonthly: n[3], KeepYearly: n[4]}, true
}

// ZFSReplica is how a ZFS item replicates.
type ZFSReplica struct {
	// TargetKind is one of the ZFSReplicaTarget kinds. TargetID names a
	// ZFSReplicaServer for a server and the pairing member id of the instance
	// that receives for a peer.
	TargetKind string
	TargetID   string
	// AfterBackup runs the replica after each backup of the item. Without it
	// Cadence schedules it.
	AfterBackup bool
	Cadence     string
	Keep        ZFSReplicaKeep
	// Peer is what the receiving instance of a peer target answered. Only
	// SetZFSReplicaPeer writes it, and a new target clears it.
	Peer ZFSReplicaPeer
	// Folder is the folder below a target's root that the members land in,
	// fixed by the first member that arrived. Neither a new target nor
	// switching the replica off clears it, so a renamed instance goes on
	// writing where it started.
	Folder string
}

// ZFSReplicaPeer is a peer target's answer to this instance's request: its
// state and, once allowed, the receive slot the streams go to.
type ZFSReplicaPeer struct {
	// State is one of the ZFSReceive states, empty before the first answer.
	State string
	Slot  string
	// TokenEnc is the slot's bearer token sealed with the APP_KEY.
	TokenEnc []byte
	// Base is where the members land there, <root>/<server name>, and URL
	// where that instance answers the slot routes.
	Base string
	URL  string
	// Pin is the SHA-256 of the public key that instance serves TLS with, in
	// hex, and empty when it serves plain HTTP.
	Pin string
}

// DefaultZFSReplica is the setting of an item that has never been given one.
func DefaultZFSReplica() ZFSReplica {
	return ZFSReplica{TargetKind: ZFSReplicaTargetNone, AfterBackup: true, Keep: DefaultZFSReplicaKeep}
}

// ZFSReplicaServer is a ZFS host that items replicate to over SSH. It needs no
// BombVault of its own.
type ZFSReplicaServer struct {
	ID   string
	Name string
	Host string
	User string
	Port int
	// Root is the dataset on Pool that holds one folder per source instance.
	Pool    string
	Root    string
	Enabled bool
	// KeyDir holds the connection's key and known_hosts. Only Create and
	// SetZFSReplicaServerKeyDir write it.
	KeyDir    string
	CreatedAt int64
}

// ZFSReplicaState is where one member of an item stands on its target: the
// newest replica snapshot both sides hold, so a run can start from it without
// listing every snapshot first.
type ZFSReplicaState struct {
	ItemID     string
	Dataset    string
	TargetPath string
	Volume     bool
	// SourceBase is a snapshot, or the bookmark left once the snapshot is
	// destroyed. The guids are decimal text.
	SourceBase string
	SourceGUID string
	TargetBase string
	TargetGUID string
	// CreatedParent is set when BombVault created TargetPath as an empty
	// parent, the one case where a first full receive may use -F.
	CreatedParent bool
	UpdatedAt     int64
}

// ZFSReplicaRunMember is what one replica run did to one member.
type ZFSReplicaRunMember struct {
	RunID   string
	ItemID  string
	Dataset string
	// Base is what the increment was sent from, empty for a full send.
	Base     string
	Snapshot string
	Bytes    int64
	Seconds  int64
	Resumed  bool
	// Code is the internal/zfs reason code of a failure, empty on success.
	Code       string
	FinishedAt int64
}

// ZFSReplicaRun is a replica run with what it did to each member.
type ZFSReplicaRun struct {
	Run
	Members []ZFSReplicaRunMember
}

// ZFSReplicaImport is the replica part of a settings file: the servers, and
// how each item replicates keyed by its root dataset.
type ZFSReplicaImport struct {
	Servers []ZFSReplicaServer
	Items   map[string]ZFSReplica
}

const zfsReplicaServerCols = `id, name, host, ssh_user, port, pool, root, enabled, key_dir, created_at`

// CreateZFSReplicaServer inserts a server, assigning an ID and CreatedAt when
// they are unset, and returns the stored row.
func (r *Repo) CreateZFSReplicaServer(s ZFSReplicaServer) (ZFSReplicaServer, error) {
	if s.ID == "" {
		s.ID = newID()
	}
	if s.CreatedAt == 0 {
		s.CreatedAt = time.Now().Unix()
	}
	_, err := r.db.Exec(`INSERT INTO zfs_replica_servers (`+zfsReplicaServerCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Host, s.User, s.Port, s.Pool, s.Root, boolInt(s.Enabled), s.KeyDir, s.CreatedAt)
	if err != nil {
		return ZFSReplicaServer{}, fmt.Errorf("CreateZFSReplicaServer: %w", err)
	}
	return s, nil
}

// UpdateZFSReplicaServer writes everything the dialog edits on the server
// with s.ID. A server that now points at another host, port, pool or root is
// another place, so the items replicating there forget where they stood. A
// missing server gives sql.ErrNoRows.
func (r *Repo) UpdateZFSReplicaServer(s ZFSReplicaServer) error {
	return r.inTx(func(tx *sql.Tx) error {
		moved, err := zfsReplicaServerMoved(tx, s)
		if err != nil {
			return fmt.Errorf("UpdateZFSReplicaServer: %w", err)
		}
		res, err := tx.Exec(`UPDATE zfs_replica_servers
			SET name = ?, host = ?, ssh_user = ?, port = ?, pool = ?, root = ?, enabled = ?
			WHERE id = ?`,
			s.Name, s.Host, s.User, s.Port, s.Pool, s.Root, boolInt(s.Enabled), s.ID)
		if err != nil {
			return fmt.Errorf("UpdateZFSReplicaServer: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("UpdateZFSReplicaServer %q: %w", s.ID, sql.ErrNoRows)
		}
		if moved {
			if err := forgetZFSReplicaServerPlace(tx, s.ID); err != nil {
				return fmt.Errorf("UpdateZFSReplicaServer: %w", err)
			}
		}
		return nil
	})
}

// ZFSReplicaServerMoved reports whether s points at another place than was:
// another host, port, pool or root.
func ZFSReplicaServerMoved(was, s ZFSReplicaServer) bool {
	return was.Host != s.Host || was.Port != s.Port || was.Pool != s.Pool || was.Root != s.Root
}

// zfsReplicaServerMoved compares s with the stored server of its id. One that
// is not stored yet has not moved.
func zfsReplicaServerMoved(tx *sql.Tx, s ZFSReplicaServer) (bool, error) {
	was, err := scanZFSReplicaServer(tx.QueryRow(`SELECT `+zfsReplicaServerCols+` FROM zfs_replica_servers WHERE id = ?`, s.ID))
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return ZFSReplicaServerMoved(was, s), nil
}

// SetZFSReplicaServerKeyDir writes where the server's key and known_hosts
// live.
func (r *Repo) SetZFSReplicaServerKeyDir(id, dir string) error {
	if _, err := r.db.Exec(`UPDATE zfs_replica_servers SET key_dir = ? WHERE id = ?`, dir, id); err != nil {
		return fmt.Errorf("SetZFSReplicaServerKeyDir: %w", err)
	}
	return nil
}

// GetZFSReplicaServer returns the server with the given id, or false when
// there is none.
func (r *Repo) GetZFSReplicaServer(id string) (ZFSReplicaServer, bool, error) {
	s, err := scanZFSReplicaServer(r.db.QueryRow(`SELECT `+zfsReplicaServerCols+` FROM zfs_replica_servers WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return ZFSReplicaServer{}, false, nil
	}
	if err != nil {
		return ZFSReplicaServer{}, false, err
	}
	return s, true, nil
}

// ListZFSReplicaServers returns every server ordered by name.
func (r *Repo) ListZFSReplicaServers() ([]ZFSReplicaServer, error) {
	rows, err := r.db.Query(`SELECT ` + zfsReplicaServerCols + ` FROM zfs_replica_servers ORDER BY name, created_at`)
	if err != nil {
		return nil, fmt.Errorf("ListZFSReplicaServers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSReplicaServer
	for rows.Next() {
		s, err := scanZFSReplicaServer(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ZFSReplicaServerUsers maps each server id to the ZFS items replicating to
// it, ordered by root dataset. A server nobody uses is absent.
func (r *Repo) ZFSReplicaServerUsers() (map[string][]string, error) {
	rows, err := r.db.Query(`SELECT replica_target_id, id FROM zfs_datasets
		WHERE replica_target_kind = ? ORDER BY dataset`, ZFSReplicaTargetServer)
	if err != nil {
		return nil, fmt.Errorf("ZFSReplicaServerUsers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	out := map[string][]string{}
	for rows.Next() {
		var server, item string
		if err := rows.Scan(&server, &item); err != nil {
			return nil, fmt.Errorf("ZFSReplicaServerUsers: %w", err)
		}
		out[server] = append(out[server], item)
	}
	return out, rows.Err()
}

// DeleteZFSReplicaServer removes a server. The items replicating to it stop
// replicating and forget where they stood there; the datasets on the server
// are not touched. Deleting a missing server is not an error.
func (r *Repo) DeleteZFSReplicaServer(id string) error {
	return r.inTx(func(tx *sql.Tx) error {
		if err := detachZFSReplicaServer(tx, id); err != nil {
			return fmt.Errorf("DeleteZFSReplicaServer: %w", err)
		}
		if _, err := tx.Exec(`DELETE FROM zfs_replica_servers WHERE id = ?`, id); err != nil {
			return fmt.Errorf("DeleteZFSReplicaServer: %w", err)
		}
		return nil
	})
}

func detachZFSReplicaServer(tx *sql.Tx, id string) error {
	if err := forgetZFSReplicaServerPlace(tx, id); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE zfs_datasets SET replica_target_kind = ?, replica_target_id = ''
		WHERE replica_target_kind = ? AND replica_target_id = ?`,
		ZFSReplicaTargetNone, ZFSReplicaTargetServer, id)
	return err
}

// forgetZFSReplicaServerPlace drops where the items replicating to a server
// stand there and what their runs there did, for a server that goes away or
// now points at another place.
func forgetZFSReplicaServerPlace(tx *sql.Tx, id string) error {
	const items = `SELECT id FROM zfs_datasets WHERE replica_target_kind = ? AND replica_target_id = ?`
	if _, err := tx.Exec(`DELETE FROM zfs_replica_state WHERE item_id IN (`+items+`)`, ZFSReplicaTargetServer, id); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM zfs_replica_runs WHERE item_id IN (`+items+`)`, ZFSReplicaTargetServer, id)
	return err
}

func scanZFSReplicaServer(s scanner) (ZFSReplicaServer, error) {
	var srv ZFSReplicaServer
	var enabled int
	err := s.Scan(&srv.ID, &srv.Name, &srv.Host, &srv.User, &srv.Port, &srv.Pool, &srv.Root,
		&enabled, &srv.KeyDir, &srv.CreatedAt)
	if err != nil {
		return ZFSReplicaServer{}, fmt.Errorf("scanZFSReplicaServer: %w", err)
	}
	srv.Enabled = enabled != 0
	return srv, nil
}

// SetZFSReplicaTarget points the item at a server, at the instance that
// receives it, or with ZFSReplicaTargetNone at nothing. A different target
// starts from scratch, so the item's member state, what its runs to the old
// target did and the answer of an old peer target are dropped with it.
func (r *Repo) SetZFSReplicaTarget(id, kind, targetID string) error {
	return r.inTx(func(tx *sql.Tx) error {
		return setZFSReplicaTarget(tx, "SetZFSReplicaTarget", id, kind, targetID)
	})
}

func setZFSReplicaTarget(tx *sql.Tx, label, id, kind, targetID string) error {
	var oldKind, oldID string
	err := tx.QueryRow(`SELECT replica_target_kind, replica_target_id FROM zfs_datasets WHERE id = ?`, id).
		Scan(&oldKind, &oldID)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: no ZFS item %q", label, id)
	}
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if oldKind == kind && oldID == targetID {
		return nil
	}
	if err := deleteZFSReplicaRows(tx, id); err != nil {
		return fmt.Errorf("%s state: %w", label, err)
	}
	if _, err := tx.Exec(`UPDATE zfs_datasets SET replica_target_kind = ?, replica_target_id = ?,
		replica_peer_state = '', replica_peer_slot = '', replica_peer_token_enc = x'',
		replica_peer_base = '', replica_peer_url = '', replica_peer_pin = ''
		WHERE id = ?`, kind, targetID, id); err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	return nil
}

// SetZFSReplicaPeer records what the receiving instance of the item's peer
// target answered.
func (r *Repo) SetZFSReplicaPeer(id string, p ZFSReplicaPeer) error {
	return r.updateZFSDataset("SetZFSReplicaPeer", id, `UPDATE zfs_datasets SET replica_peer_state = ?,
		replica_peer_slot = ?, replica_peer_token_enc = ?, replica_peer_base = ?, replica_peer_url = ?, replica_peer_pin = ?
		WHERE id = ?`, p.State, p.Slot, notNullBlob(p.TokenEnc), p.Base, p.URL, p.Pin, id)
}

// FixZFSReplicaFolder records the folder the item's members land in, unless
// one is fixed already.
func (r *Repo) FixZFSReplicaFolder(id, folder string) error {
	if _, err := r.db.Exec(`UPDATE zfs_datasets SET replica_folder = ? WHERE id = ? AND replica_folder = ''`, folder, id); err != nil {
		return fmt.Errorf("FixZFSReplicaFolder: %w", err)
	}
	return nil
}

// SetZFSReplicaAfterBackup switches between replicating after each backup and
// replicating on the item's own cadence.
func (r *Repo) SetZFSReplicaAfterBackup(id string, on bool) error {
	return r.updateZFSDataset("SetZFSReplicaAfterBackup", id,
		`UPDATE zfs_datasets SET replica_after_backup = ? WHERE id = ?`, boolInt(on), id)
}

// SetZFSReplicaCadence writes the replica's own cadence, which counts while
// it does not run after each backup.
func (r *Repo) SetZFSReplicaCadence(id, cadence string) error {
	return r.updateZFSDataset("SetZFSReplicaCadence", id,
		`UPDATE zfs_datasets SET replica_cadence = ? WHERE id = ?`, cadence, id)
}

// SetZFSReplicaKeep writes what the item keeps on its target.
func (r *Repo) SetZFSReplicaKeep(id string, keep ZFSReplicaKeep) error {
	raw, err := json.Marshal(keep)
	if err != nil {
		return fmt.Errorf("SetZFSReplicaKeep marshal: %w", err)
	}
	return r.updateZFSDataset("SetZFSReplicaKeep", id,
		`UPDATE zfs_datasets SET replica_keep = ? WHERE id = ?`, string(raw), id)
}

// ImportZFSReplica writes the replica part of a settings file in one
// transaction. Servers keep this instance's key directory, a server that moved
// is treated as UpdateZFSReplicaServer treats it, servers the file does not
// carry are deleted as DeleteZFSReplicaServer does, and an item whose root
// dataset is not an item here is skipped.
func (r *Repo) ImportZFSReplica(in ZFSReplicaImport) error {
	now := time.Now().Unix()
	return r.inTx(func(tx *sql.Tx) error {
		keep := make([]string, 0, len(in.Servers))
		for _, s := range in.Servers {
			created := s.CreatedAt
			if created == 0 {
				created = now
			}
			moved, err := zfsReplicaServerMoved(tx, s)
			if err != nil {
				return fmt.Errorf("ImportZFSReplica server %s: %w", s.ID, err)
			}
			if moved {
				if err := forgetZFSReplicaServerPlace(tx, s.ID); err != nil {
					return fmt.Errorf("ImportZFSReplica server %s: %w", s.ID, err)
				}
			}
			_, err = tx.Exec(`INSERT INTO zfs_replica_servers (`+zfsReplicaServerCols+`)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', ?)
				ON CONFLICT(id) DO UPDATE SET
					name = excluded.name, host = excluded.host, ssh_user = excluded.ssh_user,
					port = excluded.port, pool = excluded.pool, root = excluded.root,
					enabled = excluded.enabled`,
				s.ID, s.Name, s.Host, s.User, s.Port, s.Pool, s.Root, boolInt(s.Enabled), created)
			if err != nil {
				return fmt.Errorf("ImportZFSReplica server %s: %w", s.ID, err)
			}
			keep = append(keep, s.ID)
		}

		ids, err := zfsReplicaServerIDs(tx)
		if err != nil {
			return fmt.Errorf("ImportZFSReplica: %w", err)
		}
		for _, id := range ids {
			if slices.Contains(keep, id) {
				continue
			}
			if err := detachZFSReplicaServer(tx, id); err != nil {
				return fmt.Errorf("ImportZFSReplica detach %s: %w", id, err)
			}
			if _, err := tx.Exec(`DELETE FROM zfs_replica_servers WHERE id = ?`, id); err != nil {
				return fmt.Errorf("ImportZFSReplica delete %s: %w", id, err)
			}
		}

		for dataset, rep := range in.Items {
			var id string
			err := tx.QueryRow(`SELECT id FROM zfs_datasets WHERE dataset = ?`, dataset).Scan(&id)
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			if err != nil {
				return fmt.Errorf("ImportZFSReplica %s: %w", dataset, err)
			}
			if err := setZFSReplicaTarget(tx, "ImportZFSReplica", id, rep.TargetKind, rep.TargetID); err != nil {
				return err
			}
			raw, err := json.Marshal(rep.Keep)
			if err != nil {
				return fmt.Errorf("ImportZFSReplica %s keep: %w", dataset, err)
			}
			if _, err := tx.Exec(`UPDATE zfs_datasets
				SET replica_after_backup = ?, replica_cadence = ?, replica_keep = ? WHERE id = ?`,
				boolInt(rep.AfterBackup), rep.Cadence, string(raw), id); err != nil {
				return fmt.Errorf("ImportZFSReplica %s: %w", dataset, err)
			}
		}
		return nil
	})
}

func zfsReplicaServerIDs(tx *sql.Tx) ([]string, error) {
	rows, err := tx.Query(`SELECT id FROM zfs_replica_servers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

const zfsReplicaStateCols = `item_id, dataset, target_path, volume, source_base, source_guid,
	target_base, target_guid, created_parent, updated_at`

// PutZFSReplicaState stores where one member stands, replacing what was
// there. An unset UpdatedAt is now.
func (r *Repo) PutZFSReplicaState(st ZFSReplicaState) error {
	if st.UpdatedAt == 0 {
		st.UpdatedAt = time.Now().Unix()
	}
	_, err := r.db.Exec(`INSERT OR REPLACE INTO zfs_replica_state (`+zfsReplicaStateCols+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ItemID, st.Dataset, st.TargetPath, boolInt(st.Volume), st.SourceBase, st.SourceGUID,
		st.TargetBase, st.TargetGUID, boolInt(st.CreatedParent), st.UpdatedAt)
	if err != nil {
		return fmt.Errorf("PutZFSReplicaState %s: %w", st.Dataset, err)
	}
	return nil
}

// GetZFSReplicaState returns where one member stands, or false when it has
// never been replicated.
func (r *Repo) GetZFSReplicaState(itemID, dataset string) (ZFSReplicaState, bool, error) {
	st, err := scanZFSReplicaState(r.db.QueryRow(`SELECT `+zfsReplicaStateCols+`
		FROM zfs_replica_state WHERE item_id = ? AND dataset = ?`, itemID, dataset))
	if errors.Is(err, sql.ErrNoRows) {
		return ZFSReplicaState{}, false, nil
	}
	if err != nil {
		return ZFSReplicaState{}, false, err
	}
	return st, true, nil
}

// ListZFSReplicaStates returns an item's members on its target, ordered by
// dataset.
func (r *Repo) ListZFSReplicaStates(itemID string) ([]ZFSReplicaState, error) {
	rows, err := r.db.Query(`SELECT `+zfsReplicaStateCols+`
		FROM zfs_replica_state WHERE item_id = ? ORDER BY dataset`, itemID)
	if err != nil {
		return nil, fmt.Errorf("ListZFSReplicaStates: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSReplicaState
	for rows.Next() {
		st, err := scanZFSReplicaState(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// DeleteZFSReplicaState forgets where one member stands, so its next run
// resolves the base from the snapshots on both sides.
func (r *Repo) DeleteZFSReplicaState(itemID, dataset string) error {
	if _, err := r.db.Exec(`DELETE FROM zfs_replica_state WHERE item_id = ? AND dataset = ?`, itemID, dataset); err != nil {
		return fmt.Errorf("DeleteZFSReplicaState: %w", err)
	}
	return nil
}

func scanZFSReplicaState(s scanner) (ZFSReplicaState, error) {
	var st ZFSReplicaState
	var volume, parent int
	err := s.Scan(&st.ItemID, &st.Dataset, &st.TargetPath, &volume, &st.SourceBase, &st.SourceGUID,
		&st.TargetBase, &st.TargetGUID, &parent, &st.UpdatedAt)
	if err != nil {
		return ZFSReplicaState{}, fmt.Errorf("scanZFSReplicaState: %w", err)
	}
	st.Volume = volume != 0
	st.CreatedParent = parent != 0
	return st, nil
}

// AddZFSReplicaRunMember records what a run did to one member. It is written
// as each member finishes, so an interrupted run keeps what it managed.
func (r *Repo) AddZFSReplicaRunMember(m ZFSReplicaRunMember) error {
	_, err := r.db.Exec(`INSERT OR REPLACE INTO zfs_replica_runs
		(run_id, item_id, dataset, base, snapshot, bytes, seconds, resumed, code, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.RunID, m.ItemID, m.Dataset, m.Base, m.Snapshot, m.Bytes, m.Seconds, boolInt(m.Resumed),
		m.Code, m.FinishedAt)
	if err != nil {
		return fmt.Errorf("AddZFSReplicaRunMember %s: %w", m.Dataset, err)
	}
	return nil
}

// ListZFSReplicaRunMembers returns one run's members ordered by dataset.
func (r *Repo) ListZFSReplicaRunMembers(runID string) ([]ZFSReplicaRunMember, error) {
	rows, err := r.db.Query(`SELECT run_id, item_id, dataset, base, snapshot, bytes, seconds, resumed, code, finished_at
		FROM zfs_replica_runs WHERE run_id = ? ORDER BY dataset`, runID)
	if err != nil {
		return nil, fmt.Errorf("ListZFSReplicaRunMembers: %w", err)
	}
	defer rows.Close() //nolint:errcheck // rows.Close on a completed query is always nil for SQLite

	var out []ZFSReplicaRunMember
	for rows.Next() {
		var m ZFSReplicaRunMember
		var resumed int
		if err := rows.Scan(&m.RunID, &m.ItemID, &m.Dataset, &m.Base, &m.Snapshot, &m.Bytes, &m.Seconds,
			&resumed, &m.Code, &m.FinishedAt); err != nil {
			return nil, fmt.Errorf("ListZFSReplicaRunMembers scan: %w", err)
		}
		m.Resumed = resumed != 0
		out = append(out, m)
	}
	return out, rows.Err()
}

// LatestZFSReplicaRun returns the newest replica run of an item, a running one
// included, or false when it has none.
func (r *Repo) LatestZFSReplicaRun(itemID string) (ZFSReplicaRun, bool, error) {
	run, err := scanRun(r.db.QueryRow(`SELECT `+runCols+` FROM runs
		WHERE target_id = ? AND kind = ?
		ORDER BY started_at DESC, rowid DESC LIMIT 1`, itemID, ZFSReplicaRunKind))
	if errors.Is(err, sql.ErrNoRows) {
		return ZFSReplicaRun{}, false, nil
	}
	if err != nil {
		return ZFSReplicaRun{}, false, fmt.Errorf("LatestZFSReplicaRun: %w", err)
	}
	members, err := r.ListZFSReplicaRunMembers(run.ID)
	if err != nil {
		return ZFSReplicaRun{}, false, err
	}
	return ZFSReplicaRun{Run: run, Members: members}, true, nil
}

// LastZFSReplicaSuccess returns when the item's last successful replica run to
// its current target finished, 0 when there is none. A new target drops the
// member detail of the runs before it, so a run without any went elsewhere.
func (r *Repo) LastZFSReplicaSuccess(itemID string) (int64, error) {
	var at sql.NullInt64
	err := r.db.QueryRow(`SELECT MAX(finished_at) FROM runs
		WHERE target_id = ? AND kind = ? AND status = 'success' AND finished_at IS NOT NULL`+sanePastStamp+`
		AND EXISTS (SELECT 1 FROM zfs_replica_runs m WHERE m.run_id = runs.id)`,
		itemID, ZFSReplicaRunKind, saneStampCutoff()).Scan(&at)
	if err != nil {
		return 0, fmt.Errorf("LastZFSReplicaSuccess: %w", err)
	}
	return at.Int64, nil
}

// deleteZFSReplicaRows drops the member state and run detail of an item.
func deleteZFSReplicaRows(tx *sql.Tx, itemID string) error {
	if _, err := tx.Exec(`DELETE FROM zfs_replica_state WHERE item_id = ?`, itemID); err != nil {
		return err
	}
	_, err := tx.Exec(`DELETE FROM zfs_replica_runs WHERE item_id = ?`, itemID)
	return err
}
