package store_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const testResource = "https://vault.example/mcp"

func addOAuthClient(t *testing.T, r *store.Repo, id string, now int64) store.OAuthClient {
	t.Helper()
	c := store.OAuthClient{
		ID:           id,
		Name:         "Client " + id,
		RedirectURIs: []string{"https://client.example/callback/" + id},
		AuthMethod:   "none",
	}
	if err := r.CreateOAuthClient(c, now); err != nil {
		t.Fatalf("CreateOAuthClient(%s): %v", id, err)
	}
	return c
}

func addGrant(t *testing.T, r *store.Repo, id, client string, now int64) store.MCPKey {
	t.Helper()
	g, err := r.CreateOAuthGrant(store.OAuthGrant{
		ID:          id,
		Label:       "Assistant",
		OAuthClient: client,
		Resource:    testResource,
		Check:       "check-" + id,
	}, now)
	if err != nil {
		t.Fatalf("CreateOAuthGrant(%s): %v", id, err)
	}
	return g
}

// addSignedIn creates a grant together with an access token valid for an hour
// and a refresh token valid for 30 days.
func addSignedIn(t *testing.T, r *store.Repo, id, client, access, refresh string, now int64) store.MCPKey {
	t.Helper()
	g, err := r.CreateOAuthGrant(store.OAuthGrant{
		ID:             id,
		Label:          "Assistant",
		OAuthClient:    client,
		Resource:       testResource,
		Check:          "check-" + id,
		AccessDigest:   access,
		RefreshDigest:  refresh,
		AccessExpires:  now + 3600,
		RefreshExpires: now + 30*86400,
	}, now)
	if err != nil {
		t.Fatalf("CreateOAuthGrant(%s): %v", id, err)
	}
	return g
}

func TestOAuthSettingsStartOffAndKeepWhatWasSaved(t *testing.T) {
	r := newMCPRepo(t)
	c, err := r.MCPOAuthSettings()
	if err != nil {
		t.Fatal(err)
	}
	if c.Enabled || c.Issuer != "" {
		t.Fatalf("a fresh database has OAuth settings %+v, want off and empty", c)
	}
	if err := r.SetMCPOAuthSettings(store.MCPOAuthSettings{Enabled: true, Issuer: "https://vault.example"}, 100); err != nil {
		t.Fatal(err)
	}
	c, err = r.MCPOAuthSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !c.Enabled || c.Issuer != "https://vault.example" || c.UpdatedAt != 100 {
		t.Fatalf("read back %+v", c)
	}
}

func TestOAuthClientKeepsItsRedirectURIs(t *testing.T) {
	r := newMCPRepo(t)
	want := store.OAuthClient{
		ID:           "c1",
		Name:         "ChatGPT",
		RedirectURIs: []string{"https://chatgpt.com/connector_platform_oauth_redirect", "http://127.0.0.1/cb"},
		AuthMethod:   "client_secret_post",
		SecretDigest: "digest",
		Known:        "chatgpt",
		CreatedFrom:  "203.0.113.4",
	}
	if err := r.CreateOAuthClient(want, 50); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetOAuthClient("c1")
	if err != nil {
		t.Fatal(err)
	}
	want.CreatedAt = 50
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("read back %+v, want %+v", got, want)
	}
	if _, err := r.GetOAuthClient("missing"); !errors.Is(err, store.ErrOAuthClientNotFound) {
		t.Fatalf("an unknown client gave %v", err)
	}
}

func TestRegistrationSpamEvictsTheOldestUnusedClient(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "used", 1)
	addGrant(t, r, "g-used", "used", 2)
	for i := 0; i < store.OAuthUnusedClientLimit; i++ {
		addOAuthClient(t, r, fmt.Sprintf("spam%03d", i), int64(10+i))
	}
	addOAuthClient(t, r, "latest", 1000)

	n, err := r.OAuthClientCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != store.OAuthUnusedClientLimit+1 {
		t.Fatalf("%d clients stored, want the unused cap plus the one in use", n)
	}
	if _, err := r.GetOAuthClient("spam000"); !errors.Is(err, store.ErrOAuthClientNotFound) {
		t.Fatalf("the oldest unused client survived the cap: %v", err)
	}
	for _, id := range []string{"used", "spam001", "latest"} {
		if _, err := r.GetOAuthClient(id); err != nil {
			t.Fatalf("client %s was dropped: %v", id, err)
		}
	}
}

func TestAClientStillSigningInSurvivesARegistrationFlood(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "chatgpt", 1000)
	for i := 0; i < store.OAuthUnusedClientLimit; i++ {
		addOAuthClient(t, r, fmt.Sprintf("spam%03d", i), 1001)
	}
	if _, err := r.GetOAuthClient("chatgpt"); err != nil {
		t.Fatalf("a client registered a second before the flood was evicted: %v", err)
	}
	addOAuthClient(t, r, "later", 1000+store.OAuthPendingClientGrace+1)
	if _, err := r.GetOAuthClient("chatgpt"); !errors.Is(err, store.ErrOAuthClientNotFound) {
		t.Fatalf("an unused client past its grace kept its place at the cap: %v", err)
	}
}

func TestAFloodPastTheCeilingEvictsEvenNewClients(t *testing.T) {
	r := newMCPRepo(t)
	for i := 0; i < store.OAuthUnusedClientCeiling; i++ {
		addOAuthClient(t, r, fmt.Sprintf("spam%04d", i), int64(1000+i/100))
	}
	addOAuthClient(t, r, "latest", 1100)
	n, err := r.OAuthClientCount()
	if err != nil {
		t.Fatal(err)
	}
	if n != store.OAuthUnusedClientCeiling {
		t.Fatalf("%d clients stored, want the ceiling", n)
	}
	if _, err := r.GetOAuthClient("spam0000"); !errors.Is(err, store.ErrOAuthClientNotFound) {
		t.Fatalf("the oldest client survived the ceiling: %v", err)
	}
}

func TestUnusedClientsExpireAndClientsWithAGrantStay(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "idle", 100)
	addOAuthClient(t, r, "kept", 100)
	addSignedIn(t, r, "g1", "kept", "a1", "r1", 200)

	if err := r.PruneOAuth(100 + store.OAuthUnusedClientTTL + 1); err != nil {
		t.Fatal(err)
	}
	if _, err := r.GetOAuthClient("idle"); !errors.Is(err, store.ErrOAuthClientNotFound) {
		t.Fatalf("an unused client outlived its day: %v", err)
	}
	if _, err := r.GetOAuthClient("kept"); err != nil {
		t.Fatalf("a client with a grant was pruned: %v", err)
	}
}

func TestGrantsHaveTheirOwnLimitBesideTheKeys(t *testing.T) {
	r := newMCPRepo(t)
	for i := 0; i < store.MCPKeyLimit; i++ {
		addMCPKey(t, r, fmt.Sprintf("key%02d", i), fmt.Sprintf("key %d", i), true, 10)
	}
	for i := 0; i < store.MCPGrantLimit; i++ {
		id := fmt.Sprintf("c%02d", i)
		addOAuthClient(t, r, id, 20)
		addGrant(t, r, "g"+id, id, 30)
	}
	addOAuthClient(t, r, "extra", 40)
	_, err := r.CreateOAuthGrant(store.OAuthGrant{ID: "gextra", Label: "Extra", OAuthClient: "extra", Resource: "r", Check: "c"}, 50)
	if !errors.Is(err, store.ErrMCPGrantLimit) {
		t.Fatalf("a grant past the cap gave %v, want ErrMCPGrantLimit", err)
	}
	keys, err := r.ActiveMCPKeys()
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != store.MCPKeyLimit {
		t.Fatalf("ActiveMCPKeys returned %d rows, want only the %d keys", len(keys), store.MCPKeyLimit)
	}
}

func TestGrantLabelsStayUniqueAmongActiveRows(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "k1", "ChatGPT", true, 10)
	addOAuthClient(t, r, "c1", 20)
	addOAuthClient(t, r, "c2", 20)
	g1, err := r.CreateOAuthGrant(store.OAuthGrant{ID: "g1", Label: "ChatGPT", OAuthClient: "c1", Resource: "r", Check: "c"}, 30)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := r.CreateOAuthGrant(store.OAuthGrant{ID: "g2", Label: "ChatGPT", OAuthClient: "c2", Resource: "r", Check: "c"}, 40)
	if err != nil {
		t.Fatal(err)
	}
	if g1.Label != "ChatGPT 2" || g2.Label != "ChatGPT 3" {
		t.Fatalf("labels %q and %q, want ChatGPT 2 and ChatGPT 3", g1.Label, g2.Label)
	}
	if g1.Kind != store.MCPKindOAuth || g1.OAuthClient != "c1" || g1.Resource != "r" || g1.Digest != "" {
		t.Fatalf("grant row %+v", g1)
	}
}

func TestSigningInAgainReplacesTheClientsEarlierGrant(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 20)
	second, err := r.CreateOAuthGrant(store.OAuthGrant{ID: "g2", Label: "Assistant", OAuthClient: "c1", Resource: "r", Check: "c"}, 30)
	if err != nil {
		t.Fatal(err)
	}
	if second.Label != "Assistant" {
		t.Fatalf("the new grant is %q; the replaced one must release its label", second.Label)
	}
	old, err := r.GetMCPKey("g1")
	if err != nil {
		t.Fatal(err)
	}
	if old.RevokedAt != 30 || old.RevokedReason != "replaced" {
		t.Fatalf("earlier grant %+v, want revoked as replaced", old)
	}
	if _, err := r.OAuthAccessGrant("a1", 25); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("the replaced grant's token still works: %v", err)
	}
}

func TestAnAccessTokenOpensOnlyItsActiveUnexpiredGrant(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)

	got, err := r.OAuthAccessGrant("a1", 200)
	if err != nil || got.ID != "g1" {
		t.Fatalf("a live access token gave %+v, %v", got, err)
	}
	for name, tc := range map[string]struct {
		digest string
		now    int64
	}{
		"expired":       {"a1", 100 + 3600},
		"refresh token": {"r1", 200},
		"unknown":       {"nope", 200},
	} {
		if _, err := r.OAuthAccessGrant(tc.digest, tc.now); !errors.Is(err, store.ErrOAuthTokenNotFound) {
			t.Errorf("%s: gave %v, want ErrOAuthTokenNotFound", name, err)
		}
	}
}

func TestRevokingAGrantStopsItsTokensAtOnce(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if err := r.RevokeMCPKey("g1", "user", 150); err != nil {
		t.Fatal(err)
	}
	if _, err := r.OAuthAccessGrant("a1", 160); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("access after revoke gave %v", err)
	}
	_, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 3600, 86400, 160)
	if !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("refresh after revoke gave %v", err)
	}
}

func TestRefreshRotationSpendsTheOldToken(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)

	got, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 200+3600, 200+86400, 200)
	if err != nil || got.ID != "g1" {
		t.Fatalf("rotation gave %+v, %v", got, err)
	}
	if _, err := r.OAuthAccessGrant("a2", 300); err != nil {
		t.Fatalf("the new access token does not work: %v", err)
	}
	if _, err := r.RotateOAuthRefresh("r2", "c1", testResource, "a3", "r3", 400+3600, 400+86400, 400); err != nil {
		t.Fatalf("the new refresh token does not work: %v", err)
	}
}

func TestAReusedRefreshTokenRevokesTheGrant(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if _, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 200+3600, 200+86400, 200); err != nil {
		t.Fatal(err)
	}

	got, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a3", "r3", 300+3600, 300+86400, 300)
	if !errors.Is(err, store.ErrOAuthRefreshReused) || got.ID != "g1" {
		t.Fatalf("replaying a spent refresh token gave %+v, %v", got, err)
	}
	row, err := r.GetMCPKey("g1")
	if err != nil {
		t.Fatal(err)
	}
	if row.RevokedAt != 300 || row.RevokedReason != "refresh-reuse" {
		t.Fatalf("grant after reuse %+v", row)
	}
	if _, err := r.OAuthAccessGrant("a2", 310); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("the thief's or the owner's access token survived reuse: %v", err)
	}
}

func TestARefreshRetriedRightAwayGetsNewTokensAndKeepsTheGrant(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if _, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 200+3600, 200+86400, 200); err != nil {
		t.Fatal(err)
	}
	retry := int64(200 + store.OAuthRefreshRetryWindow)
	if _, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a3", "r3", retry+3600, retry+86400, retry); err != nil {
		t.Fatalf("a retry within the window gave %v", err)
	}
	if _, err := r.OAuthAccessGrant("a3", retry); err != nil {
		t.Fatalf("the retry's access token does not work: %v", err)
	}
	if _, err := r.RotateOAuthRefresh("r3", "c1", testResource, "a4", "r4", 0, retry+86400, retry+60); err != nil {
		t.Fatalf("the retry's refresh token does not work: %v", err)
	}
}

func TestARefreshRetriedTooLateOrOutOfOrderRevokesTheGrant(t *testing.T) {
	for name, steps := range map[string][][2]string{
		"too late":     {{"r1", "r2"}, {"r1", "late"}},
		"out of order": {{"r1", "r2"}, {"r2", "r3"}, {"r1", "old"}},
	} {
		r := newMCPRepo(t)
		addOAuthClient(t, r, "c1", 10)
		addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
		now := int64(200)
		var err error
		for i, s := range steps {
			if s[1] == "late" {
				now += store.OAuthRefreshRetryWindow + 1
			}
			_, err = r.RotateOAuthRefresh(s[0], "c1", testResource, "a-"+s[1], s[1], now+3600, now+86400, now)
			if i < len(steps)-1 && err != nil {
				t.Fatalf("%s: step %d: %v", name, i, err)
			}
			now++
		}
		if !errors.Is(err, store.ErrOAuthRefreshReused) {
			t.Errorf("%s: gave %v, want the grant revoked", name, err)
		}
	}
}

func TestARefreshTokenWorksOnlyForItsOwnClient(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addOAuthClient(t, r, "c2", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if _, err := r.RotateOAuthRefresh("r1", "c2", testResource, "a2", "r2", 3800, 86500, 200); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("another client's refresh gave %v", err)
	}
	if _, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 3800, 86500, 200); err != nil {
		t.Fatalf("the refused attempt spent the token: %v", err)
	}
}

func TestARefreshTokenWorksOnlyForTheAddressItWasIssuedFor(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if _, err := r.RotateOAuthRefresh("r1", "c1", "https://elsewhere.example/mcp", "a2", "r2", 3800, 86500, 200); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("a refresh for another address gave %v", err)
	}
	if _, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 3800, 86500, 200); err != nil {
		t.Fatalf("the refused attempt spent the token: %v", err)
	}
}

func TestRevokingEveryGrantLeavesTheKeys(t *testing.T) {
	r := newMCPRepo(t)
	addMCPKey(t, r, "key1", "Laptop", false, 10)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	rows, err := r.RevokeOAuthGrants("oauth-off", 200)
	if err != nil || len(rows) != 1 || rows[0].ID != "g1" || rows[0].RevokedReason != "oauth-off" {
		t.Fatalf("revoked %+v, %v", rows, err)
	}
	if _, err := r.OAuthAccessGrant("a1", 210); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("the grant's access token survived: %v", err)
	}
	key, err := r.GetMCPKey("key1")
	if err != nil || key.RevokedAt != 0 {
		t.Fatalf("the key went too: %+v, %v", key, err)
	}
}

func TestAnExpiredRefreshTokenIsRefused(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	_, err := r.RotateOAuthRefresh("r1", "c1", testResource, "a2", "r2", 0, 0, 100+30*86400)
	if !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("an expired refresh token gave %v", err)
	}
}

func TestOnlyTheNewestSpentRefreshTokensAreKept(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a0", "r0", 100)
	for i := 1; i <= store.OAuthSpentRefreshKept+5; i++ {
		now := int64(100 + i)
		old := fmt.Sprintf("r%d", i-1)
		if _, err := r.RotateOAuthRefresh(old, "c1", testResource, fmt.Sprintf("a%d", i), fmt.Sprintf("r%d", i), now+3600, now+86400, now); err != nil {
			t.Fatalf("rotation %d: %v", i, err)
		}
	}
	n, err := r.OAuthTokenCount("g1")
	if err != nil {
		t.Fatal(err)
	}
	// The live refresh token, the kept spent ones, and the current and the
	// previous access token.
	want := 1 + store.OAuthSpentRefreshKept + 2
	if n != want {
		t.Fatalf("%d token rows for one grant, want %d", n, want)
	}
	last := fmt.Sprintf("a%d", store.OAuthSpentRefreshKept+5)
	prev := fmt.Sprintf("a%d", store.OAuthSpentRefreshKept+4)
	for _, a := range []string{last, prev} {
		if _, err := r.OAuthAccessGrant(a, 200); err != nil {
			t.Fatalf("access token %s stopped working: %v", a, err)
		}
	}
}

func TestPruneRevokesAGrantWhoseRefreshTokenRanOut(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addOAuthClient(t, r, "c2", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	addSignedIn(t, r, "g2", "c2", "a2", "r2", 100+20*86400)

	if err := r.PruneOAuth(100 + 30*86400 + 1); err != nil {
		t.Fatal(err)
	}
	row, err := r.GetMCPKey("g1")
	if err != nil {
		t.Fatal(err)
	}
	if row.RevokedAt == 0 || row.RevokedReason != "expired" {
		t.Fatalf("idle grant %+v, want revoked as expired", row)
	}
	row, err = r.GetMCPKey("g2")
	if err != nil {
		t.Fatal(err)
	}
	if row.RevokedAt != 0 {
		t.Fatalf("a grant with a live refresh token was revoked: %+v", row)
	}
}

func TestRevokeOAuthTokenEndsTheGrantForARefreshToken(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)

	if _, err := r.RevokeOAuthToken("a1", "c1", 110); err != nil {
		t.Fatal(err)
	}
	if _, err := r.OAuthAccessGrant("a1", 120); !errors.Is(err, store.ErrOAuthTokenNotFound) {
		t.Fatalf("a revoked access token works: %v", err)
	}
	row, _ := r.GetMCPKey("g1")
	if row.RevokedAt != 0 {
		t.Fatal("revoking an access token revoked the grant")
	}

	if _, err := r.RevokeOAuthToken("r1", "c2", 130); err != nil {
		t.Fatal(err)
	}
	row, _ = r.GetMCPKey("g1")
	if row.RevokedAt != 0 {
		t.Fatal("another client revoked this client's grant")
	}

	grant, err := r.RevokeOAuthToken("r1", "c1", 140)
	if err != nil || grant.ID != "g1" {
		t.Fatalf("revoking the refresh token gave %+v, %v", grant, err)
	}
	row, _ = r.GetMCPKey("g1")
	if row.RevokedAt != 140 || row.RevokedReason != "client" {
		t.Fatalf("grant after the client revoked it %+v", row)
	}
}

func TestRestoreRevokeDropsEveryToken(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addSignedIn(t, r, "g1", "c1", "a1", "r1", 100)
	if _, err := r.RevokeAllMCPKeys("config-restore", 150); err != nil {
		t.Fatal(err)
	}
	n, err := r.OAuthTokenCount("g1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d tokens survived the restore revoke", n)
	}
}

func TestAGrantCannotBeRotatedLikeAKey(t *testing.T) {
	r := newMCPRepo(t)
	addOAuthClient(t, r, "c1", 10)
	addGrant(t, r, "g1", "c1", 20)
	if _, err := r.RotateMCPKey("g1", "digest", "hint", "check", 30); !errors.Is(err, store.ErrMCPKeyNotFound) {
		t.Fatalf("rotating a grant gave %v", err)
	}
}
