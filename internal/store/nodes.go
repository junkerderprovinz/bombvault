package store

import (
	"database/sql"
	"errors"
	"fmt"
)

// Node is a server home runs backups on, as home keeps it.
type Node struct {
	// ID is the id the node goes by on its link.
	ID   string
	Name string
	// AppKeyEnc is the node's APP_KEY, sealed under this instance's.
	AppKeyEnc []byte
	// RelayMode and RelayURL are the relay the node was told to use.
	RelayMode string
	RelayURL  string
	CreatedAt int64
	// AdoptedAt is when the node first took this instance as its home, zero
	// before. AdoptionDue says the node answered as one set up anew, which
	// home has still to adopt again and give back what it had.
	AdoptedAt   int64
	AdoptionDue bool
	// State is NodeReleasing while home lets the node go and the node has not
	// heard it yet, empty otherwise.
	State string
	// Placement is where home sends the node's backups, "receiver" or
	// "location", empty before it was placed. PlacementDue says the node has
	// still to hear a change to it.
	Placement    string
	PlacementDue bool
	// PlacementTarget is the receiver's address for nodes, or the id of the
	// storage location, the node was sent to. PlacementSignIn is the saved
	// sign-in the node uses at that location.
	PlacementTarget string
	PlacementSignIn string
	// ContainersCadence and FilesCadence are when home starts the backups of
	// each section on the node, and ContainersStartedAt and FilesStartedAt
	// when it last did.
	ContainersCadence   string
	FilesCadence        string
	ContainersStartedAt int64
	FilesStartedAt      int64
	// SeenAt is when home last reached the node.
	SeenAt int64
	// PrunedWindow is when the last prune window opened that home pruned the
	// node's repositories in.
	PrunedWindow int64
}

// Cadence is when home starts the backups of section domain on the node.
func (n Node) Cadence(domain string) string {
	switch domain {
	case "containers":
		return n.ContainersCadence
	case "files":
		return n.FilesCadence
	}
	return ""
}

// StartedAt is when home last started the backups of section domain.
func (n Node) StartedAt(domain string) int64 {
	switch domain {
	case "containers":
		return n.ContainersStartedAt
	case "files":
		return n.FilesStartedAt
	}
	return 0
}

// nodeSection checks that domain is a section a node backs up, which is
// also the prefix of its columns.
func nodeSection(domain string) (string, error) {
	if domain != "containers" && domain != "files" {
		return "", fmt.Errorf("a node backs up no section %q", domain)
	}
	return domain, nil
}

// NodeReleasing is the state of a node home let go that has not heard it.
const NodeReleasing = "releasing"

// ErrNodeNotFound is returned for an id no node has.
var ErrNodeNotFound = errors.New("no such node")

const nodeCols = `id, name, app_key_enc, relay_mode, relay_url, created_at, adopted_at, state, placement, placement_due, placement_target,
	containers_cadence, files_cadence, containers_started_at, files_started_at, seen_at, pruned_window, adoption_due, placement_sign_in`

func scanNode(row interface{ Scan(...any) error }) (Node, error) {
	var n Node
	var due, adoptionDue int
	err := row.Scan(&n.ID, &n.Name, &n.AppKeyEnc, &n.RelayMode, &n.RelayURL, &n.CreatedAt, &n.AdoptedAt, &n.State, &n.Placement, &due, &n.PlacementTarget,
		&n.ContainersCadence, &n.FilesCadence, &n.ContainersStartedAt, &n.FilesStartedAt, &n.SeenAt, &n.PrunedWindow, &adoptionDue, &n.PlacementSignIn)
	n.PlacementDue, n.AdoptionDue = due != 0, adoptionDue != 0
	return n, err
}

// AddNode stores a new node.
func (r *Repo) AddNode(n Node) error {
	if _, err := r.db.Exec(`INSERT INTO nodes (`+nodeCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.AppKeyEnc, n.RelayMode, n.RelayURL, n.CreatedAt, n.AdoptedAt, n.State, n.Placement, boolInt(n.PlacementDue), n.PlacementTarget,
		n.ContainersCadence, n.FilesCadence, n.ContainersStartedAt, n.FilesStartedAt, n.SeenAt, n.PrunedWindow, boolInt(n.AdoptionDue), n.PlacementSignIn); err != nil {
		return fmt.Errorf("AddNode: %w", err)
	}
	return nil
}

// ListNodes returns every node, oldest first.
func (r *Repo) ListNodes() ([]Node, error) {
	rows, err := r.db.Query(`SELECT ` + nodeCols + ` FROM nodes ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("ListNodes: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	out := []Node{}
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("ListNodes: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// GetNode returns one node.
func (r *Repo) GetNode(id string) (Node, error) {
	n, err := scanNode(r.db.QueryRow(`SELECT `+nodeCols+` FROM nodes WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Node{}, ErrNodeNotFound
	}
	if err != nil {
		return Node{}, fmt.Errorf("GetNode: %w", err)
	}
	return n, nil
}

// SetNodeAdopted records when a node took this instance as its home. A node
// adopted again keeps the time of its first adoption.
func (r *Repo) SetNodeAdopted(id string, at int64) error {
	res, err := r.db.Exec(`UPDATE nodes SET adopted_at = CASE adopted_at WHEN 0 THEN ? ELSE adopted_at END WHERE id = ?`, at, id)
	if err != nil {
		return fmt.Errorf("SetNodeAdopted: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// SetNodeState records the state of a node.
func (r *Repo) SetNodeState(id, state string) error {
	res, err := r.db.Exec(`UPDATE nodes SET state = ? WHERE id = ?`, state, id)
	if err != nil {
		return fmt.Errorf("SetNodeState: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// SetNodePlaced records where the node's backups go, now that it heard it.
func (r *Repo) SetNodePlaced(id, placement, target, signIn string) error {
	return r.updateNode(`UPDATE nodes SET placement = ?, placement_target = ?, placement_sign_in = ?, placement_due = 0 WHERE id = ?`, placement, target, signIn, id)
}

// SetNodeAdoptionDue records that an adopted node home has not let go
// answered as one set up anew, so that home adopts it again and sends it its
// placement. It reports whether this call was the one to record it.
func (r *Repo) SetNodeAdoptionDue(id string) (bool, error) {
	res, err := r.db.Exec(`UPDATE nodes SET adoption_due = 1, placement_due = placement <> '' OR placement_due
		WHERE id = ? AND adopted_at <> 0 AND state = '' AND adoption_due = 0`, id)
	if err != nil {
		return false, fmt.Errorf("SetNodeAdoptionDue: %w", err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SetNodeAdoptedAgain records that a node set up anew has been adopted again
// and got back what it had.
func (r *Repo) SetNodeAdoptedAgain(id string) error {
	return r.updateNode(`UPDATE nodes SET adoption_due = 0 WHERE id = ?`, id)
}

// SetNodePlacementDue records that the node has still to hear a change to
// where its backups go.
func (r *Repo) SetNodePlacementDue(id string) error {
	return r.updateNode(`UPDATE nodes SET placement_due = 1 WHERE id = ?`, id)
}

func (r *Repo) updateNode(query string, args ...any) error {
	res, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("update node: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNodeNotFound
	}
	return nil
}

// SetNodeCadence records when home starts the backups of section domain on
// the node.
func (r *Repo) SetNodeCadence(id, domain, cadence string) error {
	col, err := nodeSection(domain)
	if err != nil {
		return err
	}
	return r.updateNode(`UPDATE nodes SET `+col+`_cadence = ? WHERE id = ?`, cadence, id)
}

// SetNodeStarted records that home started the backups of section domain on
// the node at at.
func (r *Repo) SetNodeStarted(id, domain string, at int64) error {
	col, err := nodeSection(domain)
	if err != nil {
		return err
	}
	return r.updateNode(`UPDATE nodes SET `+col+`_started_at = ? WHERE id = ?`, at, id)
}

// SetNodeSeen records that home reached the node at at.
func (r *Repo) SetNodeSeen(id string, at int64) error {
	return r.updateNode(`UPDATE nodes SET seen_at = ? WHERE id = ?`, at, id)
}

// SetNodePruned records the prune window home pruned the node's
// repositories in.
func (r *Repo) SetNodePruned(id string, window int64) error {
	return r.updateNode(`UPDATE nodes SET pruned_window = ? WHERE id = ?`, window, id)
}

// DeleteNode forgets a node with what home learned of its runs.
func (r *Repo) DeleteNode(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("DeleteNode: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op once committed
	for _, q := range []string{`DELETE FROM node_runs WHERE node_id = ?`, `DELETE FROM node_episodes WHERE node_id = ?`, `DELETE FROM nodes WHERE id = ?`} {
		if _, err := tx.Exec(q, id); err != nil {
			return fmt.Errorf("DeleteNode: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("DeleteNode: %w", err)
	}
	return nil
}

// NodeRun is one run of a node as home last learned it.
type NodeRun struct {
	NodeID string
	ID     string
	// Domain is the section, "containers" or "files", and Target the entry.
	Domain     string
	Target     string
	Status     string
	StartedAt  int64
	FinishedAt int64
	Error      string
}

// NodeRunsKept is how many runs of each node home keeps.
const NodeRunsKept = 500

// SaveNodeRuns records what home learned of runs of node id, in place of
// what it knew of them, and keeps the newest NodeRunsKept.
func (r *Repo) SaveNodeRuns(id string, runs []NodeRun) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("SaveNodeRuns: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // a no-op once committed
	for _, run := range runs {
		if _, err := tx.Exec(`INSERT INTO node_runs (node_id, id, domain, target, status, started_at, finished_at, error) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (node_id, id) DO UPDATE SET domain = excluded.domain, target = excluded.target, status = excluded.status,
				started_at = excluded.started_at, finished_at = excluded.finished_at, error = excluded.error`,
			id, run.ID, run.Domain, run.Target, run.Status, run.StartedAt, run.FinishedAt, run.Error); err != nil {
			return fmt.Errorf("SaveNodeRuns: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM node_runs WHERE node_id = ? AND id NOT IN (
		SELECT id FROM node_runs WHERE node_id = ? ORDER BY started_at DESC, id DESC LIMIT ?)`, id, id, NodeRunsKept); err != nil {
		return fmt.Errorf("SaveNodeRuns: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("SaveNodeRuns: %w", err)
	}
	return nil
}

// NodeRuns returns what home knows of the runs of node id, newest first.
func (r *Repo) NodeRuns(id string) ([]NodeRun, error) {
	rows, err := r.db.Query(`SELECT node_id, id, domain, target, status, started_at, finished_at, error FROM node_runs
		WHERE node_id = ? ORDER BY started_at DESC, id DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("NodeRuns: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	var out []NodeRun
	for rows.Next() {
		var run NodeRun
		if err := rows.Scan(&run.NodeID, &run.ID, &run.Domain, &run.Target, &run.Status, &run.StartedAt, &run.FinishedAt, &run.Error); err != nil {
			return nil, fmt.Errorf("NodeRuns: %w", err)
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

// NodeEpisodes returns the alerts about node id whose episode is not over,
// each with when it went out.
func (r *Repo) NodeEpisodes(id string) (map[string]int64, error) {
	rows, err := r.db.Query(`SELECT episode, since FROM node_episodes WHERE node_id = ?`, id)
	if err != nil {
		return nil, fmt.Errorf("NodeEpisodes: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	out := map[string]int64{}
	for rows.Next() {
		var ep string
		var since int64
		if err := rows.Scan(&ep, &since); err != nil {
			return nil, fmt.Errorf("NodeEpisodes: %w", err)
		}
		out[ep] = since
	}
	return out, rows.Err()
}

// OpenNodeEpisode records that an alert about node id went out at since.
func (r *Repo) OpenNodeEpisode(id, episode string, since int64) error {
	if _, err := r.db.Exec(`INSERT OR IGNORE INTO node_episodes (node_id, episode, since) VALUES (?, ?, ?)`, id, episode, since); err != nil {
		return fmt.Errorf("OpenNodeEpisode: %w", err)
	}
	return nil
}

// CloseNodeEpisode records that what an alert about node id said is over.
func (r *Repo) CloseNodeEpisode(id, episode string) error {
	if _, err := r.db.Exec(`DELETE FROM node_episodes WHERE node_id = ? AND episode = ?`, id, episode); err != nil {
		return fmt.Errorf("CloseNodeEpisode: %w", err)
	}
	return nil
}

// NodeLink is a node's record of its home.
type NodeLink struct {
	HomeID string
	// AdoptedAt is when home adopted this node, zero before.
	AdoptedAt int64
	// ReleasedAt is when home let this node go, zero while it has not.
	ReleasedAt int64
}

// GetNodeLink returns this node's record of its home.
func (r *Repo) GetNodeLink() (NodeLink, error) {
	var l NodeLink
	err := r.db.QueryRow(`SELECT home_id, adopted_at, released_at FROM node_link WHERE id = 1`).Scan(&l.HomeID, &l.AdoptedAt, &l.ReleasedAt)
	if err != nil {
		return NodeLink{}, fmt.Errorf("GetNodeLink: %w", err)
	}
	return l, nil
}

// SetNodeLinkAdopted records that home adopted this node.
func (r *Repo) SetNodeLinkAdopted(homeID string, at int64) error {
	if _, err := r.db.Exec(`UPDATE node_link SET home_id = ?, adopted_at = ?, released_at = 0 WHERE id = 1`, homeID, at); err != nil {
		return fmt.Errorf("SetNodeLinkAdopted: %w", err)
	}
	return nil
}

// SetNodeLinkReleased records that home let this node go.
func (r *Repo) SetNodeLinkReleased(at int64) error {
	if _, err := r.db.Exec(`UPDATE node_link SET released_at = ? WHERE id = 1`, at); err != nil {
		return fmt.Errorf("SetNodeLinkReleased: %w", err)
	}
	return nil
}

// NodePlacement is where a node sends one section's backups, as home told it.
type NodePlacement struct {
	Domain string
	Repo   string
	// CredsEnc is the sign-in for Repo, sealed under the node's APP_KEY.
	CredsEnc []byte
	// CAPEM is the certificate the node trusts Repo's server by, empty for
	// one the system trusts.
	CAPEM string
	// Kept marks a repository behind the node's own rest-server.
	Kept  bool
	SetAt int64
}

// SetNodePlacement stores where a section's backups go, in place of what was
// there.
func (r *Repo) SetNodePlacement(p NodePlacement) error {
	_, err := r.db.Exec(`INSERT INTO node_placements (domain, repo, creds_enc, ca_pem, kept, set_at) VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (domain) DO UPDATE SET repo = excluded.repo, creds_enc = excluded.creds_enc, ca_pem = excluded.ca_pem, kept = excluded.kept, set_at = excluded.set_at`,
		p.Domain, p.Repo, notNullBlob(p.CredsEnc), p.CAPEM, p.Kept, p.SetAt)
	if err != nil {
		return fmt.Errorf("SetNodePlacement: %w", err)
	}
	return nil
}

// NodePlacements returns where each section's backups go, by section.
func (r *Repo) NodePlacements() ([]NodePlacement, error) {
	rows, err := r.db.Query(`SELECT domain, repo, creds_enc, ca_pem, kept, set_at FROM node_placements ORDER BY domain`)
	if err != nil {
		return nil, fmt.Errorf("NodePlacements: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only cursor
	var out []NodePlacement
	for rows.Next() {
		var p NodePlacement
		if err := rows.Scan(&p.Domain, &p.Repo, &p.CredsEnc, &p.CAPEM, &p.Kept, &p.SetAt); err != nil {
			return nil, fmt.Errorf("NodePlacements: %w", err)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
