package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/keydeposit"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func recordsAnswer(t *testing.T, w interface{ Bytes() []byte }) nodeRecordsAnswer {
	t.Helper()
	var ans nodeRecordsAnswer
	if err := json.Unmarshal(w.Bytes(), &ans); err != nil {
		t.Fatalf("answer %s: %v", w.Bytes(), err)
	}
	return ans
}

func TestANodeHandsHomeItsRecordsOnlyWhileAdopted(t *testing.T) {
	n := newNodeUnderTest(t)
	if w := n.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{}); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("records before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	if w := n.call(t, keydeposit.ProofNodeAPI, nodeRecordsPath, nodeRecordsCall{}); w.Code != http.StatusForbidden {
		t.Errorf("a request passed off as asking for the records = %d %s", w.Code, w.Body)
	}
	w := n.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{})
	ans := recordsAnswer(t, w.Body)
	if w.Code != http.StatusOK || !ans.OK || len(ans.Records) == 0 {
		t.Fatalf("records once adopted = %d %s", w.Code, w.Body)
	}
	if again := recordsAnswer(t, n.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{Have: recordsSum(ans.Records)}).Body); !again.OK || !again.Same || len(again.Records) != 0 {
		t.Errorf("records home holds already = %+v, want them not sent again", again)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{}); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("records after the release = %d %s", w.Code, w.Body)
	}
}

func TestANodeTakesItsRecordsBackOnlyWhileAdopted(t *testing.T) {
	giver := newNodeUnderTest(t)
	giver.adopt(t)
	if _, err := giver.node.st.CreateFileSet(store.FileSet{Name: "Docs", Path: "docs", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	back := nodeRecordsBack{Records: recordsAnswer(t, giver.call(t, keydeposit.ProofNodeRecords, nodeRecordsPath, nodeRecordsCall{}).Body).Records}

	n := newNodeUnderTest(t)
	if w := n.call(t, keydeposit.ProofNodeRecordsBack, nodeRecordsBackPath, back); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeNotAdopted)) {
		t.Errorf("records back before the adoption = %d %s", w.Code, w.Body)
	}
	n.adopt(t)
	if w := n.call(t, keydeposit.ProofNodeRecords, nodeRecordsBackPath, back); w.Code != http.StatusForbidden {
		t.Errorf("asking for the records passed off as giving them back = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRecordsBack, nodeRecordsBackPath, nodeRecordsBack{Records: []byte("not gzip")}); w.Code != http.StatusBadRequest {
		t.Errorf("records that do not read = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRecordsBack, nodeRecordsBackPath, back); w.Code != http.StatusOK {
		t.Fatalf("records back once adopted = %d %s", w.Code, w.Body)
	}
	if sets, err := n.node.st.ListFileSets(); err != nil || len(sets) != 1 || sets[0].Name != "Docs" {
		t.Errorf("the node has %+v %v, want the folder set it got back", sets, err)
	}
	if w := n.call(t, keydeposit.ProofNodeRelease, nodeReleasePath, nil); w.Code != http.StatusOK {
		t.Fatalf("release = %d %s", w.Code, w.Body)
	}
	if w := n.call(t, keydeposit.ProofNodeRecordsBack, nodeRecordsBackPath, back); w.Code != http.StatusConflict || !bytes.Contains(w.Body.Bytes(), []byte(codeNodeReleased)) {
		t.Errorf("records back after the release = %d %s", w.Code, w.Body)
	}
}
