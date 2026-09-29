package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

// fleetStatusResponse is what a member answers on GET /api/group/peer/status
// and a Fleet poll decodes. Domains has the shape of GET /api/status, so the
// Fleet page reuses the dashboard's rendering.
type fleetStatusResponse struct {
	OK           bool                `json:"ok"`
	InstanceName string              `json:"instanceName"`
	Version      string              `json:"version"`
	Domains      []DomainStatusEntry `json:"domains"`
	// RemoteViewEnabled is the peer's own remote-view switch, so the Fleet
	// card can offer or grey out Open without a call that would only fail.
	RemoteViewEnabled bool `json:"remoteViewEnabled"`
}

// syncFleetPeers gives every reachable member a Fleet row. A row from before
// pairing that reported the member's name is taken over instead of adding a
// second one, which is how an old peer stops needing to be paired again.
func (s *Service) syncFleetPeers() ([]store.FleetPeer, error) {
	rows, err := s.store.ListFleetPeers()
	if err != nil {
		return nil, fmt.Errorf("list fleet peers: %w", err)
	}
	known := map[string]int{}
	for i, p := range rows {
		if p.MemberID != "" {
			known[p.MemberID] = i
		}
	}
	for _, m := range s.pairing().Members() {
		if i, ok := known[m.ID]; ok {
			if rows[i].Name != m.Name {
				rows[i].Name = m.Name
				if err := s.store.UpdateFleetPeer(rows[i]); err != nil {
					return nil, err
				}
			}
			continue
		}
		adopted := false
		for i, p := range rows {
			if p.NeedsPairing() && sameInstanceName(p, m.Name) {
				rows[i].MemberID, rows[i].Name = m.ID, m.Name
				if err := s.store.UpdateFleetPeer(rows[i]); err != nil {
					return nil, err
				}
				known[m.ID] = i
				adopted = true
				break
			}
		}
		if adopted {
			continue
		}
		p, err := s.store.CreateFleetPeer(store.FleetPeer{MemberID: m.ID, Name: m.Name, Enabled: true, SortOrder: len(rows)})
		if err != nil {
			return nil, err
		}
		known[m.ID] = len(rows)
		rows = append(rows, p)
	}
	return rows, nil
}

func sameInstanceName(p store.FleetPeer, name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && (strings.EqualFold(p.LastPollInstanceName, name) || strings.EqualFold(p.Name, name))
}

// pollAndRecordFleetPeer asks one member for its scorecard and stores the
// outcome, keeping the last good scorecard on failure.
func (s *Service) pollAndRecordFleetPeer(ctx context.Context, p store.FleetPeer) (fleetStatusResponse, error) {
	var resp fleetStatusResponse
	err := errors.New("pair this instance again: it is not in this group")
	if !p.NeedsPairing() {
		err = s.callMember(ctx, p.MemberID, http.MethodGet, "/api/group/peer/status", nil, &resp)
	}

	ok := sql.NullBool{Valid: true, Bool: err == nil}
	detail := ""
	domainsJSON := p.LastPollDomainsJSON
	instanceName := p.LastPollInstanceName
	version := p.LastPollVersion
	remoteViewEnabled := p.LastPollRemoteViewEnabled
	if err != nil {
		detail = scrubError(err)
		if len(detail) > 200 {
			detail = detail[:200]
		}
	} else {
		instanceName = resp.InstanceName
		version = resp.Version
		remoteViewEnabled = resp.RemoteViewEnabled
		if b, mErr := json.Marshal(resp.Domains); mErr == nil {
			domainsJSON = string(b)
		}
	}
	if uErr := s.store.UpdateFleetPeerPollResult(p.ID, time.Now().Unix(), ok, detail, instanceName, version, domainsJSON, remoteViewEnabled); uErr != nil {
		log.Printf("api: fleet: record poll result for %q: %v", p.Name, uErr)
	}
	return resp, err
}

// RunFleetPolls is the scheduler's fleet job: it polls every enabled member
// once and records each result. Only failing to list the rows is an error; an
// unreachable member is a recorded outcome.
func (s *Service) RunFleetPolls(ctx context.Context) error {
	peers, err := s.syncFleetPeers()
	if err != nil {
		return err
	}
	for _, p := range peers {
		if !p.Enabled || p.NeedsPairing() {
			continue
		}
		_, _ = s.pollAndRecordFleetPeer(ctx, p) //nolint:errcheck,gosec // recorded per peer, not surfaced to the sweep caller
	}
	return nil
}
