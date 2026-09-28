package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// TestReceiverCreateSavesWaitingInsteadOfRefusing checks that creating a
// received repo saves the row even when the location cannot open yet, the
// normal state for a rest-server not deployed yet or a sender that has not
// made its first copy.
func TestReceiverCreateSavesWaitingInsteadOfRefusing(t *testing.T) {
	receiver := newInstance(t, "cellar", strings.Repeat("a3", 32))
	sender := newInstance(t, "attic", strings.Repeat("b4", 32))
	pairThroughRelay(t, receiver, sender)

	code, out := receiver.do(t, http.MethodPost, "/api/receiver/repos", map[string]any{
		"name": "from attic", "repo": "rest:http://192.168.1.9:8000/attic-not-deployed-yet", "memberId": sender.id(t),
	})
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("create of a repo that cannot open yet must still save: %d %v", code, out)
	}
	repos, err := receiver.st.ListReceivedRepos()
	if err != nil || len(repos) != 1 {
		t.Fatalf("received repos = %+v, %v", repos, err)
	}
	if !repos[0].Waiting {
		t.Fatalf("a repo that cannot yet open must save waiting: %+v", repos[0])
	}

	code, out = receiver.do(t, http.MethodGet, "/api/receiver/repos", nil)
	if code != http.StatusOK || out["ok"] != true {
		t.Fatalf("list: %d %v", code, out)
	}
	list, _ := out["repos"].([]any)
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}
	row, _ := list[0].(map[string]any)
	if row["waiting"] != true || row["reachable"] != false {
		t.Fatalf("a waiting row must list waiting and unreachable: %v", row)
	}
}

// restEnvRecordingEngine simulates an append-only rest-server behind
// htpasswd: RepoOpens only succeeds when Mode.Env carries the expected
// RESTIC_REST_USERNAME/PASSWORD pair, the way a real rest-server would answer
// 401 to a request without them.
type restEnvRecordingEngine struct {
	ResticEngine
	wantUser, wantPass string
	gotEnv             []string
}

func (e *restEnvRecordingEngine) RepoOpens(_ context.Context, _ string, m restic.Mode) bool {
	e.gotEnv = m.Env
	got := map[string]string{}
	for _, kv := range m.Env {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			got[kv[:i]] = kv[i+1:]
		}
	}
	return got["RESTIC_REST_USERNAME"] == e.wantUser && got["RESTIC_REST_PASSWORD"] == e.wantPass
}

// TestReceiverOpenSendsStoredRESTCredentials checks that receiverOpen
// presents the rest-server's own login when one is sealed onto the row,
// which a password-protected rest-server requires before it will even discuss
// the repository's encryption.
func TestReceiverOpenSendsStoredRESTCredentials(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	svc := receiverTestService(appKey)
	eng := &restEnvRecordingEngine{wantUser: "bombvault-containers", wantPass: "s3cr3t"}
	svc.engine = eng

	restEnc, err := secret.Encrypt(appKey, []byte("s3cr3t"))
	if err != nil {
		t.Fatal(err)
	}
	rr := makeReceivedRepo(t, appKey, strings.Repeat("b", 64), "rest:http://box:8000/vault", 0)
	rr.RESTUser = "bombvault-containers"
	rr.RESTPasswordEnc = restEnc

	if _, _, err := svc.receiverOpen(context.Background(), rr); err != nil {
		t.Fatalf("receiverOpen with the right rest-server login must succeed: %v", err)
	}
	if len(eng.gotEnv) != 2 {
		t.Fatalf("receiverOpen must send the rest-server login as Env, got %v", eng.gotEnv)
	}
}

// A repo picked off the sender's list, as scrubRepoLocation leaves it, carries
// no credential at all: without one stored on the row, receiverOpen must fail
// rather than quietly try the rest-server with nothing.
func TestReceiverOpenWithoutStoredRESTCredentialsIsRefused(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	svc := receiverTestService(appKey)
	svc.engine = &restEnvRecordingEngine{wantUser: "bombvault-containers", wantPass: "s3cr3t"}

	rr := makeReceivedRepo(t, appKey, strings.Repeat("b", 64), "rest:http://box:8000/vault", 0)
	if _, _, err := svc.receiverOpen(context.Background(), rr); err == nil {
		t.Fatal("receiverOpen without the rest-server login must fail, not silently open")
	}
}

// A repo with no rest-server in front of it (a plain path, or one left open on
// a trusted LAN) still opens with no Env at all: RESTUser/RESTPasswordEnc
// unset must not force credentials nothing asked for.
func TestReceiverOpenWithNoRESTLoginConfiguredSendsNoEnv(t *testing.T) {
	appKey := strings.Repeat("a", 64)
	svc := receiverTestService(appKey)
	eng := &restEnvRecordingEngine{}
	svc.engine = eng

	rr := makeReceivedRepo(t, appKey, strings.Repeat("b", 64), "rest:http://box:8000/vault", 0)
	if _, _, err := svc.receiverOpen(context.Background(), rr); err != nil {
		t.Fatalf("an unauthenticated rest-server must still open: %v", err)
	}
	if len(eng.gotEnv) != 0 {
		t.Fatalf("no rest-server login was configured, but Env carried %v", eng.gotEnv)
	}
}

// TestReceiverListPromotesWaitingRepoOnceSnapshotsArrive checks that a
// waiting row turns active on its own once the receiver actually sees a
// snapshot, without the admin editing it by hand.
func TestReceiverListPromotesWaitingRepoOnceSnapshotsArrive(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	repo := seedReceivedRepo(t, sendingKey)
	h, st := receiverHandlerFixture(t, appKey)
	rr := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	rr.Waiting = true
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.handleListReceiverRepos(w, httptest.NewRequest(http.MethodGet, "/api/receiver/repos", nil))
	resp := decodeResp(t, w)
	repos, _ := resp["repos"].([]any)
	if len(repos) != 1 {
		t.Fatalf("want 1 repo, got %d", len(repos))
	}
	row, _ := repos[0].(map[string]any)
	if row["waiting"] != false {
		t.Fatalf("a repo with snapshots must promote out of waiting on list: %v", row)
	}
	got, _, err := st.GetReceivedRepo(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Waiting {
		t.Fatal("the promotion must persist, not just decorate this one response")
	}
}

// A repo that opens but has never received a single snapshot stays waiting:
// "the sender has not copied in yet" is exactly that state, not an error.
func TestReceiverListKeepsAnEmptyRepoWaiting(t *testing.T) {
	if _, err := exec.LookPath("restic"); err != nil {
		t.Skip("no restic")
	}
	appKey := strings.Repeat("ab", 32)
	sendingKey := strings.Repeat("cd", 32)
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	m := restic.Mode{Encrypted: true, Password: restickey.Derive(sendingKey)}
	if err := (restic.Restic{Bin: "restic"}).Init(context.Background(), repo, m); err != nil {
		t.Fatalf("Init: %v", err)
	}
	h, st := receiverHandlerFixture(t, appKey)
	rr := makeReceivedRepo(t, appKey, sendingKey, repo, 0)
	rr.Waiting = true
	created, err := st.CreateReceivedRepo(rr)
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.handleListReceiverRepos(w, httptest.NewRequest(http.MethodGet, "/api/receiver/repos", nil))
	resp := decodeResp(t, w)
	repos, _ := resp["repos"].([]any)
	row, _ := repos[0].(map[string]any)
	if row["waiting"] != true {
		t.Fatalf("an initialized but still-empty repo must stay waiting: %v", row)
	}
	got, _, err := st.GetReceivedRepo(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Waiting {
		t.Fatal("an empty repo must not have been promoted")
	}
}

// An already-active receiver caught mid-outage by the save's probe keeps its
// place: it already had its first snapshot, so "unreachable" is the right
// word for it, not "waiting for the first copy" again.
func TestReceiverUpdateNeverRegressesAnActiveRepoIntoWaiting(t *testing.T) {
	h, st := receiverHandlerFixture(t, strings.Repeat("ab", 32))
	created, err := st.CreateReceivedRepo(store.ReceivedRepo{
		Name: "A", Repo: filepath.Join(t.TempDir(), "does-not-exist"),
		MemberID: "member-a", Enabled: true, Waiting: false,
	})
	if err != nil {
		t.Fatal(err)
	}

	body, err := json.Marshal(map[string]any{"name": "A", "repo": created.Repo, "deadManHours": 30})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	req := jsonReq(http.MethodPut, "/api/receiver/repos/"+created.ID, bytes.NewReader(body))
	req.SetPathValue("id", created.ID)
	h.handleUpdateReceiverRepo(w, req)
	resp := decodeResp(t, w)
	if resp["ok"] != true {
		t.Fatalf("an unreachable but already-active repo must still save: %v", resp)
	}
	repoResp, _ := resp["repo"].(map[string]any)
	if repoResp["waiting"] != false {
		t.Fatalf("an already-active repo must not regress to waiting on a transient outage: %v", repoResp)
	}
	got, _, err := st.GetReceivedRepo(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Waiting {
		t.Fatal("the stored row must not have regressed to waiting")
	}
}

// The rest-server login is exactly as sensitive as the restic password: a
// stored row must never echo either back over the API, waiting or not.
func TestReceiverListNeverLeaksTheRESTPassword(t *testing.T) {
	appKey := strings.Repeat("ab", 32)
	h, st := receiverHandlerFixture(t, appKey)
	restEnc, err := secret.Encrypt(appKey, []byte("s3cr3t"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateReceivedRepo(store.ReceivedRepo{
		Name: "A", Repo: "rest:https://box/vault", ResticPasswordEnc: []byte("ciphertext"),
		RESTUser: "bombvault-containers", RESTPasswordEnc: restEnc, Enabled: true, Waiting: true,
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	h.handleListReceiverRepos(w, httptest.NewRequest(http.MethodGet, "/api/receiver/repos", nil))
	resp := decodeResp(t, w)
	repos, _ := resp["repos"].([]any)
	row, _ := repos[0].(map[string]any)
	for _, field := range []string{"appKey", "resticPassword", "resticPasswordEnc", "restPassword", "restPasswordEnc"} {
		if _, leaked := row[field]; leaked {
			t.Fatalf("the list carries %s", field)
		}
	}
}
