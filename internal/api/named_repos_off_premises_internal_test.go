package api

import "testing"

func TestNewNamedRepositoryStandsOffThePremisesWhenRemote(t *testing.T) {
	f := newPlacementFixture(t)
	remote := f.do("POST", "/api/repos", map[string]any{"name": "Storagebox", "repo": "sftp:u@box:/bv"})
	local := f.do("POST", "/api/repos", map[string]any{"name": "NAS Keller", "repo": "nas/bv"})
	if got := remote["repo"].(map[string]any)["offPremises"]; got != true {
		t.Fatalf("a remote repository starts off the premises, got %v", got)
	}
	if got := local["repo"].(map[string]any)["offPremises"]; got != false {
		t.Fatalf("a mounted folder starts on the premises, got %v", got)
	}
}

func TestOffPremisesCanBeSwitchedOffForARestServerInTheBuilding(t *testing.T) {
	f := newPlacementFixture(t)
	created := f.do("POST", "/api/repos", map[string]any{"name": "LAN rest", "repo": "rest:http://nas:8000/bv"})
	id := created["repo"].(map[string]any)["id"].(string)
	if m := f.do("PATCH", "/api/repos/"+id, map[string]any{"offPremises": false}); m["ok"] != true {
		t.Fatalf("PATCH offPremises: %v", m)
	}
	got, err := f.st.GetNamedRepo(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.OffPremises {
		t.Fatal("the mark did not switch off")
	}
	listed := f.do("GET", "/api/repos", nil)["repos"].([]any)[0].(map[string]any)
	if listed["offPremises"] != false {
		t.Fatalf("GET /api/repos shows %v", listed["offPremises"])
	}
}

func TestDirectRepositoryRefusesTheOffPremisesMark(t *testing.T) {
	f := newPlacementFixture(t)
	direct := f.direct(f.target("containers", "B2", "b2:bucket:containers"))
	if m := f.do("PATCH", "/api/repos/"+direct.ID, map[string]any{"offPremises": true}); m["ok"] != false || m["code"] != "mirrored-field" {
		t.Fatalf("PATCH offPremises on a direct repository = %v, want mirrored-field", m)
	}
	vms := f.target("vms", "B2 vms", "b2:bucket:vms")
	m := f.do("POST", "/api/repos", map[string]any{
		"name": "", "repo": "b2:bucket:vms-direct", "companionOf": vms.ID, "offPremises": true,
	})
	if m["ok"] != false || m["code"] != "mirrored-field" {
		t.Fatalf("POST a direct repository with offPremises = %v, want mirrored-field", m)
	}
}

func TestMovingARepositoryMarksItFromItsNewLocation(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.offPremises(box.ID)
	f.makeRepo(f.root + "/backups/cold")

	if m := f.do("PATCH", "/api/repos/"+box.ID, map[string]any{"repo": "backups/cold"}); m["ok"] != true {
		t.Fatalf("PATCH repo: %v", m)
	}
	moved, err := f.st.GetNamedRepo(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Repo != "backups/cold" || moved.OffPremises {
		t.Fatalf("after the move: %q, off premises %v; want the array and no mark", moved.Repo, moved.OffPremises)
	}

	m := f.do("PATCH", "/api/repos/"+box.ID, map[string]any{"repo": "sftp:u@box:/bv", "offPremises": false})
	if m["ok"] != true {
		t.Fatalf("PATCH repo and mark: %v", m)
	}
	back, err := f.st.GetNamedRepo(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if back.Repo != "sftp:u@box:/bv" || back.OffPremises {
		t.Fatalf("a move that answers for itself: %q, off premises %v", back.Repo, back.OffPremises)
	}
	if got := m["repo"].(map[string]any)["offPremises"]; got != false {
		t.Fatalf("the answer says %v", got)
	}
}

func TestImportMovingAKnownRepositoryMarksItFromTheNewLocation(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.offPremises(box.ID)
	f.makeRepo(f.root + "/backups/cold")

	exp := f.do("GET", "/api/settings/export", nil)
	for _, row := range exp["namedRepos"].([]any) {
		r := row.(map[string]any)
		if r["id"] == box.ID {
			r["repo"] = "backups/cold"
			delete(r, "offPremises") // a file written before the field existed
		}
	}
	if m := f.do("POST", "/api/settings/import?apply=true", exp); m["ok"] != true {
		t.Fatalf("import: %v", m)
	}
	moved, err := f.st.GetNamedRepo(box.ID)
	if err != nil {
		t.Fatal(err)
	}
	if moved.Repo != "backups/cold" || moved.OffPremises {
		t.Fatalf("after the import: %q, off premises %v; want the array and no mark", moved.Repo, moved.OffPremises)
	}
}

func TestImportTakesTheOffPremisesMarkFromTheFileOrKeepsTheStoredOne(t *testing.T) {
	f := newPlacementFixture(t)
	box := f.namedRepo("Storagebox", "sftp:u@box:/bv")
	f.offPremises(box.ID)
	lan := f.namedRepo("LAN rest", "rest:http://nas:8000/bv")
	nas := f.namedRepo("NAS Keller", "sftp:u@nas:/bv")

	exp := f.do("GET", "/api/settings/export", nil)
	rows := exp["namedRepos"].([]any)
	for _, row := range rows {
		r := row.(map[string]any)
		switch r["id"] {
		case box.ID, lan.ID:
			delete(r, "offPremises") // a file written before the field existed
		case nas.ID:
			r["offPremises"] = true
		}
	}
	exp["namedRepos"] = append(rows, map[string]any{
		"id": "0123456789abcdef0123456789abcdef", "name": "Archive", "repo": "b2:bucket:archive", "enabled": true,
	})
	if m := f.do("POST", "/api/settings/import?apply=true", exp); m["ok"] != true {
		t.Fatalf("import: %v", m)
	}

	expected := map[string]bool{box.ID: true, lan.ID: false, nas.ID: true, "0123456789abcdef0123456789abcdef": true}
	for id, want := range expected {
		got, err := f.st.GetNamedRepo(id)
		if err != nil {
			t.Fatal(err)
		}
		if got.OffPremises != want {
			t.Errorf("%s (%s): offPremises = %v, want %v", got.Name, got.Repo, got.OffPremises, want)
		}
	}
}
