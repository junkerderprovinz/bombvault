package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/nodelink"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// Where home asks a node for its records and gives them back to the node set
// up anew.
const (
	nodeRecordsPath     = "/node/records"
	nodeRecordsBackPath = "/node/records/back"
)

// nodeRecordsRuns is how many of its newest finished runs a node keeps with
// home.
const nodeRecordsRuns = 200

// maxNodeRecords caps a node's records, compressed, so that they go back to
// it in one call over the relay.
const maxNodeRecords = 512 << 10

// maxNodeRecordsCall caps the call that gives them back.
const maxNodeRecordsCall = 1 << 20

var (
	errNodeRecordsSize = errors.New("the node's containers, folder sets and runs are too large for home to keep")
	errNodeRecordsRead = errors.New("the records home gave back could not be read")
)

// nodeRecordsCall is home asking a node for its records. Have is the sum of
// the ones home holds, which the node does not send again.
type nodeRecordsCall struct {
	Have string `json:"have,omitempty"`
}

// nodeRecordsAnswer is a node's records as JSON compressed with gzip, or
// Same when home holds them as they are.
type nodeRecordsAnswer struct {
	OK      bool   `json:"ok"`
	Same    bool   `json:"same,omitempty"`
	Records []byte `json:"records,omitempty"`
}

// nodeRecordsBack is home giving a node back the records it kept of it.
type nodeRecordsBack struct {
	Records []byte `json:"records"`
}

// recordsSum is the sum home and node compare records by, empty for none.
func recordsSum(records []byte) string {
	if len(records) == 0 {
		return ""
	}
	sum := sha256.Sum256(records)
	return hex.EncodeToString(sum[:])
}

// records answers home with this node's records, unless home holds them
// already.
func (n *nodeSide) records(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodeRecords)
	if !ok || !n.adopted(w) {
		return
	}
	var req nodeRecordsCall
	if json.Unmarshal(call.Body, &req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "the call could not be read"})
		return
	}
	records, err := n.s.ownRecords()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	if recordsSum(records) == req.Have {
		writeJSON(w, http.StatusOK, nodeRecordsAnswer{OK: true, Same: true})
		return
	}
	writeJSON(w, http.StatusOK, nodeRecordsAnswer{OK: true, Records: records})
}

// ownRecords is this node's records as JSON compressed with gzip, which
// writes the same bytes for the same records.
func (s *Service) ownRecords() ([]byte, error) {
	rec, err := s.store.OwnRecords(nodeRecordsRuns)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return nil, err
	}
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(raw); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	if zipped.Len() > maxNodeRecords {
		return nil, errNodeRecordsSize
	}
	return zipped.Bytes(), nil
}

// recordsBack puts back what this node lacks of the records home gives it.
func (n *nodeSide) recordsBack(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signedUpTo(w, r, keydeposit.ProofNodeRecordsBack, maxNodeRecordsCall)
	if !ok || !n.adopted(w) {
		return
	}
	rec, err := readRecords(call.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if err := n.s.store.RestoreOwnRecords(rec); err != nil {
		writeJSON(w, http.StatusInternalServerError, failEnvelope(err))
		return
	}
	log.Printf("node: home gave back %d containers, %d folder sets and %d runs", len(rec.Targets), len(rec.FileSets), len(rec.Runs))
	writeJSON(w, http.StatusOK, okEnvelope(nil))
}

// readRecords reads the records of a call that gives them back.
func readRecords(body []byte) (store.OwnRecords, error) {
	var req nodeRecordsBack
	var rec store.OwnRecords
	if json.Unmarshal(body, &req) != nil {
		return rec, errNodeRecordsRead
	}
	zr, err := gzip.NewReader(bytes.NewReader(req.Records))
	if err != nil {
		return rec, errNodeRecordsRead
	}
	defer zr.Close() //nolint:errcheck // reading from memory
	raw, err := io.ReadAll(io.LimitReader(zr, pageLimit))
	if err != nil || json.Unmarshal(raw, &rec) != nil {
		return rec, errNodeRecordsRead
	}
	return rec, nil
}

// keepNodeRecords asks node n for its records, unless home holds them as
// they are, and keeps what it answers. A node due to be adopted again is not
// asked, since what it has is what home is about to give back to it.
func (s *Service) keepNodeRecords(ctx context.Context, n store.Node) error {
	if n.AdoptionDue {
		return nil
	}
	held, err := s.heldNodeRecords(n.ID)
	if err != nil {
		return err
	}
	var ans nodeRecordsAnswer
	if err := s.callNodeByID(ctx, n.ID, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{Have: recordsSum(held)}, &ans); err != nil {
		return err
	}
	if ans.Same || len(ans.Records) == 0 {
		return nil
	}
	return s.holdNodeRecords(n.ID, ans.Records)
}

// holdNodeRecords keeps the records of node id sealed under this instance's
// APP_KEY.
func (s *Service) holdNodeRecords(id string, records []byte) error {
	sealed, err := secret.Encrypt(s.cfg.AppKey, records)
	if err != nil {
		return err
	}
	return s.store.KeepNodeRecords(id, sealed)
}

// heldNodeRecords opens the records home keeps of node id, nil for none.
func (s *Service) heldNodeRecords(id string) ([]byte, error) {
	sealed, err := s.store.NodeRecords(id)
	if err != nil || len(sealed) == 0 {
		return nil, err
	}
	return secret.Decrypt(s.cfg.AppKey, sealed)
}

// giveBackRecords hands node id the records home keeps of it and notes that
// it has what it had. Records that no longer open are left out, so the node
// still gets back to its backups.
func (s *Service) giveBackRecords(ctx context.Context, id string, link *nodelink.Link) error {
	sealed, err := s.store.NodeRecords(id)
	if err != nil {
		return err
	}
	var records []byte
	if len(sealed) > 0 {
		if records, err = secret.Decrypt(s.cfg.AppKey, sealed); err != nil {
			log.Printf("nodes: %s: the records home kept do not open, so the node starts without them: %v", id, err)
		}
	}
	if len(records) > 0 {
		if err := s.callNode(ctx, link, keydeposit.ProofNodeRecordsBack, id, nodeRecordsBackPath, nodeRecordsBack{Records: records}, nil); err != nil {
			return err
		}
	}
	return s.store.SetNodeAdoptedAgain(id)
}
