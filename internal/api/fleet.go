package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/group"
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
	// DirectURL is where the answering instance takes direct calls, so a
	// poll over the relay doubles as the address exchange: whichever side
	// calls the other first hands its own address along for free.
	DirectURL string `json:"directUrl,omitempty"`
	// Capabilities names what this instance can do for a member beyond what
	// its version line says, such as keeping a key deposit.
	Capabilities []string `json:"capabilities,omitempty"`
}

// syncFleetPeers gives every reachable member a Fleet row. A row from before
// pairing that reported the member's name is taken over instead of adding a
// second one, which is how an old peer stops needing to be paired again.
func (s *Service) syncFleetPeers() ([]store.FleetPeer, error) {
	return s.syncFleetPeersWith(s.pairing().Members(), time.Now())
}

// syncFleetPeersWith is syncFleetPeers for the members in reach at now, which
// it records as seen. The pages and the phones' calls run it side by side,
// and two runs at once would each add a row for a member new to both.
func (s *Service) syncFleetPeersWith(members []group.Member, now time.Time) ([]store.FleetPeer, error) {
	s.fleetSyncMu.Lock()
	defer s.fleetSyncMu.Unlock()
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
	// A removed phone still holds the group's words, so it is back as soon
	// as it is in reach, and only a person switches it on again.
	removed, err := s.store.RemovedPhones()
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if i, ok := known[m.ID]; ok {
			if err := s.store.MarkFleetPeerSeen(m.ID, now.Unix()); err != nil {
				return nil, err
			}
			// A member found on the network and never polled would otherwise
			// leave no address to ask after a restart whether it is still in
			// the group.
			if m.Kind == "" && m.Address != "" && rows[i].MemberURL != m.Address {
				if err := s.store.SetFleetPeerMemberURL(m.ID, m.Address); err != nil {
					return nil, err
				}
				rows[i].MemberURL = m.Address
			}
			if (m.Name != "" && rows[i].Name != m.Name) || (m.Kind != "" && rows[i].Kind != m.Kind) {
				if m.Name != "" {
					rows[i].Name = m.Name
				}
				if m.Kind != "" {
					rows[i].Kind = m.Kind
				}
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
				if err := s.store.MarkFleetPeerSeen(m.ID, now.Unix()); err != nil {
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
		row := store.FleetPeer{MemberID: m.ID, Name: m.Name, Kind: m.Kind, Enabled: !(m.Kind == "android" && removed[m.ID]), SortOrder: len(rows), LastSeen: now.Unix()}
		if m.Kind == "" {
			row.MemberURL = m.Address
		}
		p, err := s.store.CreateFleetPeer(row)
		if err != nil {
			return nil, err
		}
		known[m.ID] = len(rows)
		rows = append(rows, p)
	}
	return rows, nil
}

// awayMember is a member of the group with a row here that is not in reach
// now, such as an instance that was shut down or a phone with the app closed,
// as it last said who it is.
type awayMember struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind,omitempty"`
	Address string `json:"address"`
	// LastSeen is Unix seconds.
	LastSeen int64 `json:"lastSeen"`
}

// groupRoster returns the members in reach, the ones out of reach that keep
// their row until somebody removes it, and whether the group may still be
// coming back after a restart or a change. A member gets its row by being in
// reach, which only one that proved it holds the group's words can be.
func (s *Service) groupRoster(now time.Time) (members []group.Member, away []awayMember, settling bool, err error) {
	members = s.pairing().Members()
	away = []awayMember{}
	if !s.pairing().Active() {
		return members, away, false, nil
	}
	rows, err := s.syncFleetPeersWith(members, now)
	if err != nil {
		return nil, nil, false, err
	}
	present := map[string]bool{}
	for _, m := range members {
		present[m.ID] = true
	}
	for _, p := range rows {
		if p.MemberID == "" || present[p.MemberID] {
			continue
		}
		away = append(away, awayMember{ID: p.MemberID, Name: p.Name, Kind: p.Kind, Address: p.MemberURL, LastSeen: lastSeen(p)})
	}
	sort.Slice(away, func(i, j int) bool {
		if away[i].Name != away[j].Name {
			return away[i].Name < away[j].Name
		}
		return away[i].ID < away[j].ID
	})
	settling = now.Sub(time.Unix(0, s.groupApplied.Load())) < absentCheckDelay
	return members, away, settling, nil
}

// lastSeen is the latest a row knows the member was there. A row older than
// the last_seen column still knows when it was made and when a poll was
// answered.
func lastSeen(p store.FleetPeer) int64 {
	seen := max(p.LastSeen, p.CreatedAt)
	if p.LastPollOK.Valid && p.LastPollOK.Bool {
		seen = max(seen, p.LastPollAt)
	}
	return seen
}

// memberPresent reports whether the member is reachable in the group at the moment.
func (s *Service) memberPresent(id string) error {
	for _, m := range s.pairing().Members() {
		if m.ID == id {
			return nil
		}
	}
	return group.ErrNotMember
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
	switch {
	case p.NeedsPairing():
	case p.Kind != "":
		// The app has no scorecard; being on the relay is all there is to know.
		err = s.memberPresent(p.MemberID)
	default:
		err = s.callMember(ctx, p.MemberID, http.MethodGet, "/api/group/peer/status", nil, &resp)
		if err == nil {
			// The key pinned while the member is here is what its word that it
			// left the group is checked against later. A member too old to sign
			// has none.
			_, _ = s.memberKey(ctx, p.MemberID) //nolint:errcheck // see above
		}
	}

	ok := sql.NullBool{Valid: true, Bool: err == nil}
	detail := ""
	domainsJSON := p.LastPollDomainsJSON
	instanceName := p.LastPollInstanceName
	version := p.LastPollVersion
	if err != nil {
		detail = scrubError(err)
		if len(detail) > 200 {
			detail = detail[:200]
		}
	} else {
		instanceName = resp.InstanceName
		version = resp.Version
		if b, mErr := json.Marshal(resp.Domains); mErr == nil {
			domainsJSON = string(b)
		}
	}
	if uErr := s.store.UpdateFleetPeerPollResult(p.ID, time.Now().Unix(), ok, detail, instanceName, version, domainsJSON); uErr != nil {
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

// A member that left the group while this instance was down never tells it,
// and one that is out of reach is not asked by anything else until the daily
// Fleet sweep. absentCheckDelay gives the relay and the network time to bring
// back the members still in the group before the first check after a start.
const (
	absentCheckDelay   = 30 * time.Second
	absentCheckEvery   = 5 * time.Minute
	absentCheckTimeout = 10 * time.Second
)

func (s *Service) absentMembersLoop() {
	ctx := s.StopContext()
	wait := absentCheckDelay
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		s.askAbsentMembers(ctx)
		wait = absentCheckEvery
	}
}

// askAbsentMembers asks every member that has a row and an address but is not
// in reach now, and drops the row of one that answers it left the group.
// Nothing else of the answer is recorded; polls do that.
func (s *Service) askAbsentMembers(ctx context.Context) {
	if !s.pairing().Active() {
		return
	}
	rows, err := s.store.ListFleetPeers()
	if err != nil {
		log.Printf("group: %v", err)
		return
	}
	present := map[string]bool{}
	for _, m := range s.pairing().Members() {
		present[m.ID] = true
	}
	for _, p := range rows {
		if p.MemberID == "" || p.Kind != "" || p.MemberURL == "" || present[p.MemberID] {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, absentCheckTimeout)
		_, _, err := s.pairing().Call(cctx, p.MemberID, http.MethodGet, group.ProbePath, nil)
		cancel()
		if left, ok := errors.AsType[*group.LeftError](err); ok {
			s.dropFormerMember(left)
		}
	}
}
