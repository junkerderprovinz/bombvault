//go:build flows

package flows

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// pairedInstances is two installs in one group, through a relay the first one
// serves, so nothing leaves the machine. tower holds one backup of a folder.
type pairedInstances struct {
	tower, vault     *bombvault
	towerID, vaultID string
	// towerRepo is a copy of tower's repository in vault's mount: the two share
	// no folder on this machine, as two boxes reading one NAS would.
	towerRepo string
}

func answer(b *bombvault, method, path string, in any) map[string]any {
	b.t.Helper()
	_, body := b.send(method, path, in)
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		b.t.Fatalf("%s %s: %v: %s", method, path, err, body)
	}
	return out
}

func newPairedInstances(t *testing.T) *pairedInstances {
	t.Helper()
	t.Parallel()
	p := &pairedInstances{tower: newBombvault(t), vault: newBombvault(t)}
	for name, b := range map[string]*bombvault{"tower": p.tower, "vault": p.vault} {
		b.start(currentBinary)
		b.signIn()
		b.changeSettings(func(s map[string]any) {
			s["instanceName"], s["fleetEnabled"], s["filesPath"] = name, true, "user/own/files"
			s["tamperTestSchedule"], s["digestSchedule"] = "off", "off"
		})
	}
	var phrase struct {
		Phrase string `json:"phrase"`
	}
	p.tower.call("POST", "/api/group/phrase", map[string]any{}, &phrase)
	p.tower.call("PUT", "/api/group/relay", map[string]any{"mode": "own", "url": p.tower.base, "serve": true}, nil)
	p.vault.call("POST", "/api/group/join", map[string]any{"phrase": phrase.Phrase}, nil)
	p.vault.call("PUT", "/api/group/relay", map[string]any{"mode": "own", "url": p.tower.base, "serve": false}, nil)
	p.towerID = p.memberOf(p.vault)
	p.vaultID = p.memberOf(p.tower)

	p.tower.write("user/data", "a.txt", "invented\n", 0o644)
	id := p.tower.createSet("docs", "user/data")
	known := len(p.tower.runsOf("backup"))
	p.tower.call("POST", "/api/files/sets/"+id+"/backup", nil, nil)
	if got := p.tower.awaitRun("backup", known); got.Status != "success" {
		t.Fatalf("tower's backup: %+v", got)
	}
	p.towerRepo = "user/from-tower"
	if err := os.CopyFS(p.vault.hostPath(p.towerRepo), os.DirFS(p.tower.hostPath("user/own/files"))); err != nil {
		t.Fatal(err)
	}
	return p
}

// memberOf waits for b to see the other install in its group.
func (*pairedInstances) memberOf(b *bombvault) string {
	b.t.Helper()
	for range 60 {
		var group struct {
			Members []struct {
				ID string `json:"id"`
			} `json:"members"`
		}
		b.call("GET", "/api/group", nil, &group)
		if len(group.Members) > 0 {
			return group.Members[0].ID
		}
		time.Sleep(500 * time.Millisecond)
	}
	b.t.Fatal("the group did not come together")
	return ""
}

func (p *pairedInstances) allowOnVault(role string) map[string]any {
	p.vault.t.Helper()
	var roles struct {
		Members []struct {
			ForMember map[string]*struct {
				ID string `json:"id"`
			} `json:"forMember"`
		} `json:"members"`
	}
	p.vault.call("GET", "/api/group/roles", nil, &roles)
	req := roles.Members[0].ForMember[role]
	if req == nil {
		p.vault.t.Fatalf("no %s request on vault", role)
	}
	return answer(p.vault, "POST", "/api/group/roles/requests/"+req.ID, map[string]any{"decision": "allow", "sections": []string{"files"}})
}

func (p *pairedInstances) source() map[string]any {
	return map[string]any{"name": "tower", "repo": p.towerRepo, "domain": "files", "memberId": p.towerID, "cadence": "off"}
}

func TestAFetcherIsAskedAllowedAndKeepsItsCopyWhenTheFolderMoves(t *testing.T) {
	p := newPairedInstances(t)

	p.vault.changeSettings(func(s map[string]any) { s["pullEnabled"] = true })
	early := answer(p.vault, "POST", "/api/pull/sources", p.source())
	if early["ok"] != false || !strings.Contains(fmt.Sprint(early["error"]), "has not asked") {
		t.Errorf("Add source before the other side asked: %v", early)
	}
	p.vault.changeSettings(func(s map[string]any) { s["pullEnabled"] = false })

	p.tower.call("PUT", "/api/group/members/"+p.vaultID+"/roles/fetcher", map[string]any{"sections": []string{"files"}}, nil)
	if got := p.allowOnVault("fetcher"); got["ok"] != false || !strings.Contains(fmt.Sprint(got["error"]), "Pull") {
		t.Errorf("allow with Pull off: %v", got)
	}
	if got := answer(p.vault, "POST", "/api/pull/sources", p.source()); got["ok"] != false || !strings.Contains(fmt.Sprint(got["error"]), "Pull") {
		t.Errorf("a source with Pull off: %v", got)
	}

	p.vault.changeSettings(func(s map[string]any) { s["pullEnabled"] = true })
	if got := p.allowOnVault("fetcher"); got["ok"] != true {
		t.Fatalf("allow with Pull on: %v", got)
	}
	made := answer(p.vault, "POST", "/api/pull/sources", p.source())
	if made["ok"] != true {
		t.Fatalf("a source after the allow: %v", made)
	}
	id := made["source"].(map[string]any)["id"].(string)
	run := "/api/pull/sources/" + id + "/run"
	if got := answer(p.vault, "POST", run, map[string]any{}); got["ok"] != true || got["snapshots"] != float64(1) {
		t.Fatalf("the first pull: %v", got)
	}

	p.vault.call("POST", "/api/storage/own-folders", map[string]any{"domains": []string{"files"}, "address": "user/moved", "copyOver": false}, nil)
	if got := answer(p.vault, "POST", run, map[string]any{}); got["ok"] != true || got["snapshots"] != float64(0) {
		t.Errorf("the pull after the own folder moved: %v, want nothing to carry", got)
	}
	if _, err := os.Stat(filepath.Join(p.vault.hostPath("user/moved/files/pulled"), id)); !os.IsNotExist(err) {
		t.Errorf("a second copy started in the new folder: %v", err)
	}
	if _, err := os.Stat(filepath.Join(p.vault.hostPath("user/own/files/pulled"), id, "config")); err != nil {
		t.Errorf("the copy left its place: %v", err)
	}

	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		return
	}
	unreadable := p.vault.hostPath(p.towerRepo + "/snapshots")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o755) })
	if got := answer(p.vault, "POST", run, map[string]any{}); got["ok"] != false {
		t.Errorf("a pull whose source cannot be read: %v", got)
	}
	var listed struct {
		Sources []struct {
			LastPullOK *bool `json:"lastPullOk"`
		} `json:"sources"`
	}
	p.vault.call("GET", "/api/pull/sources", nil, &listed)
	if len(listed.Sources) != 1 || listed.Sources[0].LastPullOK == nil || *listed.Sources[0].LastPullOK {
		t.Errorf("the source reads %+v after it could not be read", listed.Sources)
	}
	if failed := p.vault.runsOf("pull"); len(failed) != 1 || failed[0].Status != "failed" {
		t.Errorf("the activity log holds %+v", failed)
	}
}
