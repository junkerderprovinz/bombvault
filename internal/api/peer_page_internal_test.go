package api

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/progress"
	"github.com/junkerderprovinz/bombvault/internal/relay"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// pagesOverRelay puts attic in a group with tower, serves attic's interface
// from spa and connects a phone that holds the group's words.
func pagesOverRelay(t *testing.T, spa fstest.MapFS) (tower, attic *instance, phone *wordsHolder) {
	t.Helper()
	tower = newInstance(t, "tower", strings.Repeat("a1", 32))
	attic = newInstance(t, "attic", strings.Repeat("b2", 32))
	phrase, relayURL := groupOf(t, tower, attic)
	attic.svc.ServePhonePages(NewServer(config.Config{HTTPOnly: true}, spa, attic.router).Handler())
	phone = holdWords(t, relayURL, phrase, wordsHolderID, "android", attic)
	return tower, attic, phone
}

// page asks target for one request of a page, naming sender as the member
// it comes from, and returns the call's status with the answer.
func (w *wordsHolder) page(target, sender string, ask peerPageAsk) (int, peerPageAnswer) {
	w.t.Helper()
	raw, err := json.Marshal(ask)
	if err != nil {
		w.t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := w.client.Call(context.Background(), target, relay.ProxyCall{Method: http.MethodPost, Path: "/api/group/peer/page", Body: raw, Sender: sender})
		if err == nil {
			var out peerPageAnswer
			_ = json.Unmarshal(res.Body, &out)
			return res.Status, out
		}
		if time.Now().After(deadline) {
			w.t.Fatalf("%s %s: %v", ask.Method, ask.Path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// open fetches a request of a page whole, every part of it, unpacked.
func (w *wordsHolder) open(target, method, path string, body any) (peerPageAnswer, []byte) {
	w.t.Helper()
	ask := peerPageAsk{Method: method, Path: path, Header: map[string]string{"Content-Type": "application/json"}}
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			w.t.Fatal(err)
		}
		ask.Body = b
	}
	status, first := w.page(target, w.id, ask)
	if status != http.StatusOK || !first.OK {
		w.t.Fatalf("%s %s: the call answered %d %+v", method, path, status, first)
	}
	whole := append([]byte(nil), first.Body...)
	for len(whole) < first.Size {
		status, next := w.page(target, w.id, peerPageAsk{Part: first.Part, Offset: len(whole)})
		if status != http.StatusOK || len(next.Body) == 0 {
			w.t.Fatalf("%s %s: part at %d answered %d", method, path, len(whole), status)
		}
		whole = append(whole, next.Body...)
	}
	if first.Header["Content-Encoding"] == "gzip" {
		zr, err := gzip.NewReader(bytes.NewReader(whole))
		if err != nil {
			w.t.Fatal(err)
		}
		if whole, err = io.ReadAll(zr); err != nil {
			w.t.Fatal(err)
		}
	}
	return first, whole
}

func decoded(t *testing.T, body []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("not JSON: %.200s", body)
	}
	return out
}

var shell = fstest.MapFS{"index.html": {Data: []byte("<!doctype html><title>attic</title>")}}

func TestAPageThroughTheRelayReachesWhatTheSessionOfAPhoneDoes(t *testing.T) {
	_, attic, phone := pagesOverRelay(t, shell)
	atticID := attic.id(t)

	answer, body := phone.open(atticID, http.MethodGet, "/instances", nil)
	if answer.Status != http.StatusOK || !strings.Contains(string(body), "<title>attic</title>") {
		t.Fatalf("the shell: %d %s", answer.Status, body)
	}
	if !strings.Contains(answer.Header["Content-Security-Policy"], "default-src 'self'") {
		t.Fatalf("the shell came without its policy: %v", answer.Header)
	}
	if answer, body := phone.open(atticID, http.MethodGet, "/api/auth", nil); answer.Status != http.StatusOK || decoded(t, body)["authed"] != true {
		t.Fatalf("the page is not signed in: %d %s", answer.Status, body)
	}
	if answer, body := phone.open(atticID, http.MethodGet, "/api/status", nil); answer.Status != http.StatusOK || decoded(t, body)["ok"] != true {
		t.Fatalf("the status: %d %s", answer.Status, body)
	}

	refused := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/recovery-kit", nil},
		{http.MethodGet, "/api/settings/export?includeCredentials=1", nil},
		{http.MethodPost, "/api/login", map[string]any{"password": testPassword}},
		{http.MethodPost, "/api/auth/reset", map[string]any{}},
		{http.MethodPost, "/api/group/call", map[string]any{}},
		{http.MethodPut, "/api/settings", map[string]any{}},
		{http.MethodGet, "/metrics", nil},
	}
	check := func(when string) {
		t.Helper()
		for _, r := range refused {
			answer, body := phone.open(atticID, r.method, r.path, r.body)
			if answer.Status != http.StatusForbidden || decoded(t, body)["code"] != codeOwnLoginNeeded {
				t.Errorf("%s: %s %s answered %d %s", when, r.method, r.path, answer.Status, body)
			}
		}
	}
	check("with a password")

	if _, err := attic.st.MutateSettings(func(s *store.Settings) error {
		s.AuthPasswordHash = ""
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	check("without a password")
	if answer, _ := phone.open(atticID, http.MethodGet, "/api/status", nil); answer.Status != http.StatusOK {
		t.Fatalf("the status without a password: %d", answer.Status)
	}
}

func TestOnlyAPhoneOfTheGroupOpensPagesThroughTheRelay(t *testing.T) {
	tower, attic, phone := pagesOverRelay(t, shell)
	atticID := attic.id(t)
	ask := peerPageAsk{Method: http.MethodGet, Path: "/api/status"}

	for name, sender := range map[string]string{"an instance of the group": tower.id(t), "nobody": "", "a stranger": strings.Repeat("9", 32)} {
		if status, answer := phone.page(atticID, sender, ask); status != http.StatusForbidden || answer.Body != nil {
			t.Errorf("a page in the name of %s: %d %+v", name, status, answer)
		}
	}

	if err := attic.st.SetPhonePairing(false); err != nil {
		t.Fatal(err)
	}
	if status, _ := phone.page(atticID, phone.id, ask); status != http.StatusForbidden {
		t.Fatalf("a page with pairing phones switched off: %d", status)
	}
}

func TestALargeAnswerComesThroughTheRelayWhole(t *testing.T) {
	big := make([]byte, 3*pagePart+1234)
	if _, err := rand.Read(big); err != nil {
		t.Fatal(err)
	}
	spa := fstest.MapFS{"index.html": shell["index.html"], "assets/big.js": {Data: big}}
	_, attic, phone := pagesOverRelay(t, spa)

	answer, body := phone.open(attic.id(t), http.MethodGet, "/assets/big.js", nil)
	if answer.Part == "" {
		t.Fatal("the answer came in one call")
	}
	if answer.Status != http.StatusOK || !bytes.Equal(body, big) {
		t.Fatalf("the bundle came back %d with %d of %d bytes", answer.Status, len(body), len(big))
	}
	if !strings.Contains(answer.Header["Cache-Control"], "immutable") {
		t.Fatalf("the bundle lost its caching: %v", answer.Header)
	}
	if status, _ := phone.page(attic.id(t), phone.id, peerPageAsk{Part: answer.Part, Offset: pagePart}); status != http.StatusNotFound {
		t.Fatalf("a fetched answer was kept: %d", status)
	}
}

func TestTheProgressStreamEndsWhenItGoesThroughTheRelay(t *testing.T) {
	_, attic, phone := pagesOverRelay(t, shell)
	live := progress.NewStore()
	attic.h.SetProgress(live)
	live.Publish(progress.Event{Key: "container:nginx", Phase: "backup", Percent: 40, Active: true})

	answer, body := phone.open(attic.id(t), http.MethodGet, "/api/progress", nil)
	if answer.Status != http.StatusOK || !strings.HasPrefix(answer.Header["Content-Type"], "text/event-stream") || !strings.Contains(string(body), `"container:nginx"`) {
		t.Fatalf("the progress: %d %v %s", answer.Status, answer.Header, body)
	}
}

func TestPartsGoOnlyToTheSenderThatAskedAndExpire(t *testing.T) {
	var parts pageParts
	body := bytes.Repeat([]byte("x"), 2*pagePart)
	token, ok := parts.keep("phone", body)
	if !ok {
		t.Fatal("nothing kept")
	}
	if _, _, ok := parts.take(token, "someone else", pagePart); ok {
		t.Fatal("another member took the part")
	}
	part, size, ok := parts.take(token, "phone", pagePart)
	if !ok || size != len(body) || len(part) != pagePart {
		t.Fatalf("the last part: %v %d %d", ok, size, len(part))
	}
	if parts.held != 0 {
		t.Fatalf("%d bytes held after the last part", parts.held)
	}

	token, _ = parts.keep("phone", bytes.Repeat([]byte("y"), 4*pagePart))
	for _, offset := range []int{3 * pagePart, pagePart, 3 * pagePart, 2 * pagePart} {
		if _, _, ok := parts.take(token, "phone", offset); !ok {
			t.Fatalf("the part at %d, fetched out of order, was gone", offset)
		}
	}
	if parts.held != 0 {
		t.Fatalf("%d bytes held after every part", parts.held)
	}

	token, _ = parts.keep("phone", body)
	parts.entries[token].expires = time.Now().Add(-time.Second)
	if _, _, ok := parts.take(token, "phone", 0); ok || parts.held != 0 {
		t.Fatal("an expired answer was handed out")
	}
	for range pagePartsBudget / len(body) {
		if _, ok := parts.keep("phone", body); !ok {
			t.Fatal("refused below the budget")
		}
	}
	if _, ok := parts.keep("phone", body); ok {
		t.Fatal("kept beyond the budget")
	}
}

func TestADownloadAndAnOversizedAnswerStayOffTheRelay(t *testing.T) {
	download := &pageRecorder{header: http.Header{}}
	download.Header().Set("Content-Disposition", `attachment; filename="flash.zip"`)
	_, _ = download.Write([]byte("PK"))
	if a := download.answer(); a.Status != http.StatusForbidden || bytes.Contains(a.Body, []byte("PK")) {
		t.Fatalf("a download: %d %s", a.Status, a.Body)
	}

	big := &pageRecorder{header: http.Header{}}
	chunk := make([]byte, 1<<20)
	for range pageLimit/len(chunk) + 1 {
		_, _ = big.Write(chunk)
	}
	if a := big.answer(); a.Status != http.StatusRequestEntityTooLarge || a.Size > 1024 {
		t.Fatalf("an oversized answer: %d with %d bytes", a.Status, a.Size)
	}
}
