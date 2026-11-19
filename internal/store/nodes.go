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
	// before.
	AdoptedAt int64
	// State is NodeReleasing while home lets the node go and the node has not
	// heard it yet, empty otherwise.
	State string
}

// NodeReleasing is the state of a node home let go that has not heard it.
const NodeReleasing = "releasing"

// ErrNodeNotFound is returned for an id no node has.
var ErrNodeNotFound = errors.New("no such node")

const nodeCols = `id, name, app_key_enc, relay_mode, relay_url, created_at, adopted_at, state`

func scanNode(row interface{ Scan(...any) error }) (Node, error) {
	var n Node
	err := row.Scan(&n.ID, &n.Name, &n.AppKeyEnc, &n.RelayMode, &n.RelayURL, &n.CreatedAt, &n.AdoptedAt, &n.State)
	return n, err
}

// AddNode stores a new node.
func (r *Repo) AddNode(n Node) error {
	if _, err := r.db.Exec(`INSERT INTO nodes (`+nodeCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.Name, n.AppKeyEnc, n.RelayMode, n.RelayURL, n.CreatedAt, n.AdoptedAt, n.State); err != nil {
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

// SetNodeAdopted records when a node took this instance as its home.
func (r *Repo) SetNodeAdopted(id string, at int64) error {
	res, err := r.db.Exec(`UPDATE nodes SET adopted_at = ? WHERE id = ?`, at, id)
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

// DeleteNode forgets a node.
func (r *Repo) DeleteNode(id string) error {
	if _, err := r.db.Exec(`DELETE FROM nodes WHERE id = ?`, id); err != nil {
		return fmt.Errorf("DeleteNode: %w", err)
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
