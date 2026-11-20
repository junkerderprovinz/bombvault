package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
)

// nodeRunPath is where home starts the backups of a section on a node.
const nodeRunPath = "/node/run"

// codeNodeBusy refuses a run to a node that is backing up already.
const codeNodeBusy = "node-busy"

var errNodeRunBusy = errors.New("the node is backing up already")

// nodeRunCall is home starting the backups of one section on a node.
type nodeRunCall struct {
	Domain string `json:"domain"`
}

// nodeRunAnswer says how many entries of the section the node started
// backing up.
type nodeRunAnswer struct {
	OK      bool `json:"ok"`
	Started int  `json:"started"`
}

// run backs up a section as a scheduled round would, in the background, and
// answers how many entries it started.
func (n *nodeSide) run(w http.ResponseWriter, r *http.Request) {
	call, ok := n.signed(w, r, keydeposit.ProofNodeRun)
	if !ok || !n.adopted(w) {
		return
	}
	var req nodeRunCall
	if json.Unmarshal(call.Body, &req) != nil || !slices.Contains(nodeDomains, req.Domain) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "a node backs up its containers and folders"})
		return
	}
	ctx := WithRunOrigin(context.WithoutCancel(r.Context()), RunOrigin{Via: "home", KeyID: n.home.ID})
	started, err := n.s.backUpSection(ctx, req.Domain)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "code": codeNodeBusy, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, nodeRunAnswer{OK: true, Started: started})
}

// backUpSection starts the backups of every entry a scheduled round of
// domain takes, and returns how many there are.
func (s *Service) backUpSection(ctx context.Context, domain string) (int, error) {
	entries, err := s.roundEntries(domain)
	if err != nil || len(entries) == 0 {
		return 0, err
	}
	var started bool
	if domain == "containers" {
		started, err = s.StartBackupAll(WithScheduledBackup(ctx), entries)
	} else {
		started, err = s.StartBackupFilesAll(ctx, entries)
	}
	if err == nil && !started {
		err = errNodeRunBusy
	}
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}

// roundEntries are the entries a scheduled round of domain backs up: the
// names of the containers in the schedule, or the ids of the folder sets
// that are switched on.
func (s *Service) roundEntries(domain string) ([]string, error) {
	var out []string
	if domain == "containers" {
		targets, err := s.store.ListTargets()
		if err != nil {
			return nil, err
		}
		for _, t := range targets {
			if t.IncludeInSchedule {
				out = append(out, t.ContainerName)
			}
		}
		return out, nil
	}
	sets, err := s.store.ListFileSets()
	if err != nil {
		return nil, err
	}
	for _, fs := range sets {
		if fs.Enabled {
			out = append(out, fs.ID)
		}
	}
	return out, nil
}
