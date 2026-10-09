package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/zfs"
	"github.com/junkerderprovinz/bombvault/internal/zfsrepl"
)

// zfsReplicaMembers lists the datasets and volumes of item that replicate:
// its tree on this host without the children it excludes.
func (s *Service) zfsReplicaMembers(ctx context.Context, item store.ZFSDataset) ([]string, error) {
	if s.zfs == nil {
		return nil, &zfsrepl.Refusal{Code: "ssh-missing", Detail: "this instance has no SSH connection to its host"}
	}
	tree, err := s.zfs.Tree(ctx, item.Dataset)
	if err != nil {
		return nil, err
	}
	var members []string
	for _, e := range tree {
		excluded := false
		for _, ex := range item.ExcludedChildren {
			excluded = excluded || e.Name == ex || zfs.DescendantOf(e.Name, ex)
		}
		if !excluded {
			members = append(members, e.Name)
		}
	}
	return members, nil
}

// askZFSReceive asks the instance item replicates to for a receive slot and
// stores its answer on the item. Asking again is how the source learns a
// decision, and it keeps the slot's members in step with the item's tree.
// renew is for a person on this side asking anew, the one request that
// reopens a revoked slot.
func (s *Service) askZFSReceive(ctx context.Context, item store.ZFSDataset, renew bool) (store.ZFSReplicaPeer, error) {
	members, err := s.zfsReplicaMembers(ctx, item)
	if err != nil {
		return store.ZFSReplicaPeer{}, err
	}
	g, err := s.store.GetGroupState()
	if err != nil {
		return store.ZFSReplicaPeer{}, err
	}
	settings, err := s.store.GetSettings()
	if err != nil {
		return store.ZFSReplicaPeer{}, err
	}
	req := peerZFSReceiveRequest{
		InstanceID: g.InstanceID, Name: instanceDisplayName(settings), Item: item.ID, Dataset: item.Dataset,
		Members: members, Keep: item.Replica.Keep, Renew: renew,
	}
	var ans peerZFSReceiveAnswer
	if err := s.callMember(ctx, item.Replica.TargetID, http.MethodPost, "/api/group/peer/zfs-receive", req, &ans); err != nil {
		return store.ZFSReplicaPeer{}, err
	}
	peer := store.ZFSReplicaPeer{State: ans.State}
	switch ans.State {
	case store.ZFSReceiveAsked, store.ZFSReceiveRefused, store.ZFSReceiveRevoked:
	case store.ZFSReceiveAllowed:
		if !validRunID(ans.Slot) || ans.Token == "" || zfs.ValidateDatasetName(ans.Base) != nil {
			return store.ZFSReplicaPeer{}, errMemberUnreadable
		}
		enc, err := secret.Encrypt(s.cfg.AppKey, []byte(ans.Token))
		if err != nil {
			return store.ZFSReplicaPeer{}, err
		}
		peer.Slot, peer.TokenEnc, peer.Base, peer.URL, peer.Pin = ans.Slot, enc, ans.Base, strings.TrimRight(ans.DirectURL, "/"), ans.Pin
	default:
		return store.ZFSReplicaPeer{}, errMemberUnreadable
	}
	if err := s.store.SetZFSReplicaPeer(item.ID, peer); err != nil {
		return store.ZFSReplicaPeer{}, err
	}
	return peer, nil
}

// zfsReplicaPeerEnd asks the receiving instance of item's peer target first
// and returns the End for its receive slot. Its targets are <folder>/<member>.
// While the group cannot reach that instance, a slot allowed before is used as
// it stands.
func (s *Service) zfsReplicaPeerEnd(ctx context.Context, item store.ZFSDataset) (zfsrepl.End, error) {
	peer, err := s.askZFSReceive(ctx, item, false)
	if err != nil {
		if item.Replica.Peer.State != store.ZFSReceiveAllowed {
			return nil, &zfsrepl.Refusal{Code: "not-reached", Detail: "the receiving instance did not answer", Err: err}
		}
		log.Printf("zfs replica: asking the receiving instance of %s failed, using the slot it allowed: %v", item.Dataset, err)
		peer = item.Replica.Peer
	}
	if peer.State != store.ZFSReceiveAllowed {
		return nil, &zfsrepl.Refusal{Code: "zfs-permission", Detail: "the receiving instance has not allowed this item (" + peer.State + ")"}
	}
	if peer.URL == "" {
		return nil, &zfsrepl.Refusal{Code: "not-reached", Detail: "the receiving instance did not say where it answers"}
	}
	token, err := secret.Decrypt(s.cfg.AppKey, peer.TokenEnc)
	if err != nil {
		return nil, errors.New("the stored slot token could not be opened")
	}
	return &peerEnd{
		url: peer.URL + "/api/zfs/receive/" + peer.Slot, token: string(token),
		root: item.Dataset, hc: peerEndClient(peer.Pin),
		revoked: func() {
			if err := s.store.SetZFSReplicaPeer(item.ID, store.ZFSReplicaPeer{State: store.ZFSReceiveRevoked}); err != nil {
				log.Printf("zfs replica: %v", err)
			}
		},
	}, nil
}

// zfsReplicaPeerRequest runs after an edit that set or kept a peer target or
// changed its keep. It asks the receiving instance again unless the request
// is allowed, where only that side's keep counts. A revoked slot opens again,
// a refused one only for more members, and an asked one takes the new keep as
// its proposal.
func (s *Service) zfsReplicaPeerRequest(ctx context.Context, item store.ZFSDataset) error {
	if item.Replica.TargetKind != store.ZFSReplicaTargetPeer || item.Replica.Peer.State == store.ZFSReceiveAllowed {
		return nil
	}
	_, err := s.askZFSReceive(ctx, item, true)
	return err
}

// zfsReplicaPeerState is what the paired instance answered for the item.
func (s *Service) zfsReplicaPeerState(item store.ZFSDataset) string {
	if item.Replica.TargetKind != store.ZFSReplicaTargetPeer {
		return ""
	}
	return item.Replica.Peer.State
}

// peerEndClient talks to a receiving instance. A stream runs for as long as
// its data takes, so only connecting is bounded.
func peerEndClient(pin string) *http.Client {
	return &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		Transport: &http.Transport{
			DialContext:         (&net.Dialer{Timeout: 15 * time.Second}).DialContext,
			TLSHandshakeTimeout: 15 * time.Second,
			TLSClientConfig:     pinnedTLS(pin),
		},
	}
}

// pinnedTLS accepts the receiving instance's own self-signed certificate by
// the key pin it handed out over the group, and anything else only with a
// chain the system trusts, which is what a reverse proxy in front of it
// serves.
func pinnedTLS(pin string) *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: true, //nolint:gosec // G402: VerifyConnection checks the pin or the chain
		VerifyConnection: func(cs tls.ConnectionState) error {
			if len(cs.PeerCertificates) == 0 {
				return errors.New("the receiving instance sent no certificate")
			}
			leaf := cs.PeerCertificates[0]
			if pin != "" && spkiPin(leaf) == pin {
				return nil
			}
			pool := x509.NewCertPool()
			for _, c := range cs.PeerCertificates[1:] {
				pool.AddCert(c)
			}
			_, err := leaf.Verify(x509.VerifyOptions{DNSName: cs.ServerName, Intermediates: pool})
			return err
		},
	}
}

// peerEnd is the target End of a peer replica: the receive slot on the
// paired instance. It offers what a run needs on a target, its points, its
// state and dropping an interrupted stream, and refuses the rest; the
// receiving instance creates parents and prunes on its own.
type peerEnd struct {
	url, token string
	// root is the item's root, the first of the members a target path can
	// name.
	root    string
	hc      *http.Client
	revoked func()
}

var (
	_ zfsrepl.End         = (*peerEnd)(nil)
	_ zfsrepl.SelfPruning = (*peerEnd)(nil)
)

func (e *peerEnd) PrunesItself() {}

// member is the source dataset behind a target path, <folder>/<member>. The
// folder is the source's own idea of it; the receiving instance places the
// member under the folder it fixed for this source.
func (e *peerEnd) member(target string) (string, bool) {
	_, m, ok := strings.Cut(target, "/")
	return m, ok && (m == e.root || zfs.DescendantOf(m, e.root))
}

// above reports whether target is the folder or a level between it and the
// root, which the receiving instance creates itself.
func (e *peerEnd) above(target string) bool {
	_, m, ok := strings.Cut(target, "/")
	return !ok || zfs.DescendantOf(e.root, m)
}

func (e *peerEnd) Run(ctx context.Context, args []string) (string, error) {
	target := args[len(args)-1]
	m, isMember := e.member(target)
	switch {
	case args[1] == "get" && isMember:
		v, err := e.view(ctx, args, m)
		if err != nil {
			return "", err
		}
		token := v.ResumeToken
		if token == "" {
			token = "-"
		}
		return "type\t" + v.Type + "\nencryption\t" + v.Encryption + "\nreceive_resume_token\t" + token + "\n", nil
	case args[1] == "get" && e.above(target):
		return "type\tfilesystem\nencryption\toff\nreceive_resume_token\t-\n", nil
	case args[1] == "list" && slices.Contains(args, "snapshot,bookmark") && isMember:
		v, err := e.view(ctx, args, m)
		if err != nil {
			return "", err
		}
		var b strings.Builder
		for _, p := range v.Points {
			fmt.Fprintf(&b, "%s@%s\t%s\t%d\n", target, p.Name, p.GUID, p.CreateTxg)
		}
		return b.String(), nil
	case args[1] == "receive" && args[2] == "-A" && isMember:
		resp, err := e.do(ctx, args, http.MethodPost, "/members/"+url.PathEscape(m)+"/abort", nil)
		if err != nil {
			return "", err
		}
		return "", e.result(args, resp)
	}
	return "", &zfsrepl.Refusal{Code: "zfs-permission", Detail: "the receiving instance does this itself: " + strings.Join(args, " ")}
}

// view is one member as the slot sees it. A member the slot does not hold
// fails as not-found, the way zfs reports a missing dataset.
func (e *peerEnd) view(ctx context.Context, args []string, member string) (zfsSlotMember, error) {
	resp, err := e.do(ctx, args, http.MethodGet, "/points?member="+url.QueryEscape(member), nil)
	if err != nil {
		return zfsSlotMember{}, err
	}
	defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a read body
	var out struct {
		OK      bool            `json:"ok"`
		Code    string          `json:"code"`
		Error   string          `json:"error"`
		Members []zfsSlotMember `json:"members"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, zfsReceiveBodyMax)).Decode(&out); err != nil {
		return zfsSlotMember{}, &zfsrepl.Refusal{Code: "zfs-error", Detail: "the receiving instance answered unreadably", Err: err}
	}
	if !out.OK || len(out.Members) != 1 {
		return zfsSlotMember{}, slotError(args, out.Code, out.Error)
	}
	if !out.Members[0].Exists {
		return zfsSlotMember{}, &zfs.CmdError{Args: args, Code: "not-found", Stderr: "cannot open '" + args[len(args)-1] + "': dataset does not exist"}
	}
	return out.Members[0], nil
}

func (e *peerEnd) Send(ctx context.Context, args []string) (io.ReadCloser, func() error, error) {
	ds, snap, _ := strings.Cut(args[len(args)-1], "@")
	m, ok := e.member(ds)
	if !ok {
		return nil, nil, &zfsrepl.Refusal{Code: "zfs-permission", Detail: ds + " is not part of the receive slot"}
	}
	resp, err := e.do(ctx, args, http.MethodGet, "/members/"+url.PathEscape(m)+"/send?snapshot="+url.QueryEscape(snap), nil)
	if err != nil {
		return nil, nil, err
	}
	if resp.Header.Get("Content-Type") != "application/octet-stream" {
		return nil, nil, e.result(args, resp)
	}
	wait := func() error {
		// The trailer is only there once the body has been read to its end.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		if got := resp.Trailer.Get(zfsSendResult); got != "ok" {
			return slotError(args, got, "the receiving instance did not finish the stream")
		}
		return nil
	}
	return resp.Body, wait, nil
}

func (e *peerEnd) Receive(ctx context.Context, args []string, stream io.Reader) error {
	m, ok := e.member(args[len(args)-1])
	if !ok {
		return &zfsrepl.Refusal{Code: "zfs-permission", Detail: args[len(args)-1] + " is not part of the receive slot"}
	}
	resp, err := e.do(ctx, args, http.MethodPut, "/members/"+url.PathEscape(m), stream)
	if err != nil {
		return err
	}
	return e.result(args, resp)
}

// do sends one call to the slot. A slot that is not allowed any more is
// recorded on the item, so the source stops and says so.
func (e *peerEnd) do(ctx context.Context, args []string, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, e.url+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	resp, err := e.hc.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, &zfsrepl.Refusal{Code: "not-reached", Detail: "the receiving instance is not reachable", Err: err}
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusNotFound {
		defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a read body
		var out struct {
			State string `json:"state"`
			Code  string `json:"code"`
			Error string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, zfsReceiveBodyMax)).Decode(&out)
		if out.State == store.ZFSReceiveRevoked {
			e.revoked()
		}
		if out.Code == "" {
			out.Code = "zfs-permission"
		}
		return nil, slotError(args, out.Code, out.Error)
	}
	return resp, nil
}

// result reads the JSON envelope a slot route answers with.
func (e *peerEnd) result(args []string, resp *http.Response) error {
	defer resp.Body.Close() //nolint:errcheck // nothing to do about a failed close of a read body
	var out struct {
		OK    bool   `json:"ok"`
		Code  string `json:"code"`
		Error string `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, zfsReceiveBodyMax)).Decode(&out); err != nil {
		return &zfsrepl.Refusal{Code: "zfs-error", Detail: fmt.Sprintf("the receiving instance answered HTTP %d unreadably", resp.StatusCode), Err: err}
	}
	if !out.OK {
		return slotError(args, out.Code, out.Error)
	}
	return nil
}

// slotError carries the receiving side's reason code the way a failed zfs
// command on a host would, so the engine reacts to it alike.
func slotError(args []string, code, msg string) error {
	if code == "" {
		code = "zfs-error"
	}
	return &zfs.CmdError{Args: args, Code: code, Stderr: msg}
}
