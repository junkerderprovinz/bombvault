package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// routeBuilders are the functions that register routes from a table rather
// than a literal: the public API, which its own tokens gate, and the routes
// members call over the group channel, which never pass authGate.
var routeBuilders = []string{"mountAPIV1", "peerMux"}

// registeredRoutes reads every pattern the package registers on a mux from
// its source.
func registeredRoutes(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var parsed []*ast.File
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name) //nolint:gosec // a source file of this package
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		parsed = append(parsed, f)
	}
	return routesIn(t, fset, parsed)
}

// isServeMux reports whether a type expression is *http.ServeMux.
func isServeMux(e ast.Expr) bool {
	star, ok := e.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "http" && sel.Sel.Name == "ServeMux"
}

// isNewServeMux reports whether an expression is a call to http.NewServeMux.
func isNewServeMux(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "http" && sel.Sel.Name == "NewServeMux"
}

// muxNames are the names a function holds a *http.ServeMux under: its
// parameters and variables of that type, and the struct fields of the files.
func muxNames(fn *ast.FuncDecl, fields map[string]bool) map[string]bool {
	names := maps.Clone(fields)
	for _, p := range fn.Type.Params.List {
		if isServeMux(p.Type) {
			for _, n := range p.Names {
				names[n.Name] = true
			}
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range x.Rhs {
				if id, ok := x.Lhs[i].(*ast.Ident); ok && isNewServeMux(rhs) {
					names[id.Name] = true
				}
			}
		case *ast.ValueSpec:
			for i, n := range x.Names {
				if isServeMux(x.Type) || (i < len(x.Values) && isNewServeMux(x.Values[i])) {
					names[n.Name] = true
				}
			}
		}
		return true
	})
	return names
}

// routesIn reads the patterns registered on any *http.ServeMux in files: a
// string literal, or a literal joined to a string constant of the files.
func routesIn(t *testing.T, fset *token.FileSet, files []*ast.File) []string {
	t.Helper()
	consts := map[string]ast.Expr{}
	fields := map[string]bool{}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.GenDecl:
				if x.Tok != token.CONST {
					return true
				}
				for _, spec := range x.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, n := range vs.Names {
						if i < len(vs.Values) {
							consts[n.Name] = vs.Values[i]
						}
					}
				}
			case *ast.Field:
				if isServeMux(x.Type) {
					for _, n := range x.Names {
						fields[n.Name] = true
					}
				}
			}
			return true
		})
	}
	var value func(ast.Expr) (string, bool)
	value = func(e ast.Expr) (string, bool) {
		switch x := e.(type) {
		case *ast.BasicLit:
			s, err := strconv.Unquote(x.Value)
			return s, err == nil
		case *ast.Ident:
			if c, ok := consts[x.Name]; ok {
				return value(c)
			}
			return "", false
		case *ast.BinaryExpr:
			a, okA := value(x.X)
			b, okB := value(x.Y)
			return a + b, okA && okB && x.Op == token.ADD
		}
		return "", false
	}
	var out []string
	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			muxes := muxNames(fn, fields)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 2 {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "HandleFunc" && sel.Sel.Name != "Handle") {
					return true
				}
				var recv string
				switch x := sel.X.(type) {
				case *ast.Ident:
					recv = x.Name
				case *ast.SelectorExpr:
					recv = x.Sel.Name
				}
				if !muxes[recv] {
					return true
				}
				pattern, ok := value(call.Args[0])
				if !ok {
					if !slices.Contains(routeBuilders, fn.Name.Name) {
						t.Errorf("%s: %s registers a route this test cannot read", fset.Position(call.Pos()), fn.Name.Name)
					}
					return true
				}
				out = append(out, pattern)
				return true
			})
		}
	}
	slices.Sort(out)
	return out
}

func TestRoutesAreFoundOnAMuxOfAnyName(t *testing.T) {
	const src = `package api

type server struct{ routes *http.ServeMux }

func a() {
	m := http.NewServeMux()
	m.HandleFunc("GET /a", nil)
}

func b(r *http.ServeMux) { r.Handle("GET /b", nil) }

func (s *server) c() {
	var inner *http.ServeMux
	inner.HandleFunc("GET /c", nil)
	s.routes.HandleFunc("GET /d", nil)
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "routes.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := routesIn(t, fset, []*ast.File{f}), []string{"GET /a", "GET /b", "GET /c", "GET /d"}; !slices.Equal(got, want) {
		t.Fatalf("routes = %v, want %v", got, want)
	}
}

// pairedRefused are the routes the session of a paired phone is refused on
// purpose. A route that is on neither this list nor pairedRoutes fails
// TestEveryRouteIsDecidedForAPairedSession, so whoever adds one decides.
var pairedRefused = []string{
	// The login, its second factor and passkeys, pairing phones, tokens, MCP
	// keys and OAuth.
	"DELETE /api/auth/passkeys/{id}",
	"GET /api/auth/phone-pairing",
	"PUT /api/auth/phone-pairing",
	"DELETE /api/mcp/keys/{id}",
	"DELETE /api/tokens/{id}",
	"DELETE /api/widget/token",
	"GET /api/mcp/certificate",
	"GET /api/mcp/keys",
	"GET /api/mcp/keys/{id}/activity",
	"GET /api/oauth/authorize",
	"GET /api/tokens",
	"GET /api/tokens/{id}/activity",
	"PATCH /api/auth/passkeys/{id}",
	"PATCH /api/mcp/keys/{id}",
	"PATCH /api/tokens/{id}",
	"POST /api/auth/passkey/register/begin",
	"POST /api/auth/passkey/register/finish",
	"POST /api/auth/password",
	"POST /api/auth/totp/confirm",
	"POST /api/auth/totp/disable",
	"POST /api/auth/totp/setup",
	"POST /api/logout-all",
	"POST /api/mcp/certificate/names",
	"POST /api/mcp/keys",
	"POST /api/mcp/keys/{id}/revoke",
	"POST /api/mcp/keys/{id}/rotate",
	"POST /api/oauth/authorize",
	"POST /api/tokens",
	"POST /api/tokens/{id}/revoke",
	"POST /api/tokens/{id}/rotate",
	"POST /api/widget/token",
	"PUT /api/mcp/oauth",
	// The backup key, the recovery kit, deposits, the emergency and foreign repositories.
	"DELETE /api/emergency",
	"DELETE /api/group/deposits/{owner}",
	"GET /api/emergency",
	"GET /api/emergency/deposits",
	"GET /api/group/deposits",
	"GET /api/recovery-kit",
	"POST /api/emergency/adopt",
	"POST /api/emergency/finish",
	"POST /api/emergency/group",
	"POST /api/emergency/read",
	"POST /api/emergency/sign-ins",
	"POST /api/emergency/undo",
	"POST /api/foreign/close",
	"POST /api/foreign/container-warnings",
	"POST /api/foreign/files",
	"POST /api/foreign/open",
	"POST /api/foreign/restore",
	"POST /api/group/deposit/code",
	"POST /api/group/deposit/renew",
	"POST /api/group/deposits/releases/{id}",
	"POST /api/recovery-kit/ack",
	"POST /api/recovery-kit/read",
	// Joining and changing the group, roles, receivers, pull sources and ZFS replication peers.
	"DELETE /api/fleet/peers/{id}",
	"DELETE /api/group",
	"DELETE /api/group/members/{id}/roles/{role}",
	"DELETE /api/pull/sources/{id}",
	"DELETE /api/receiver/repos/{id}",
	"DELETE /api/receiver/server",
	"DELETE /api/receiver/server/logins/{id}",
	"DELETE /api/zfs/replica/servers/{id}",
	"GET /api/fleet/mesh-offers",
	"GET /api/pull/sources",
	"GET /api/receiver/repos",
	"GET /api/receiver/repos/{id}/inventory",
	"GET /api/receiver/server",
	"GET /api/receiver/server/template",
	"GET /api/zfs/receive/{slot}/members/{member}/send",
	"GET /api/zfs/receive/{slot}/points",
	"GET /api/zfs/replica/key",
	"GET /api/zfs/replica/local-pools",
	"GET /api/zfs/replica/servers",
	"PATCH /api/zfs/receive/requests/{id}",
	"PATCH /api/zfs/replica/servers/{id}",
	"POST /api/fleet/mesh-offers/{id}/accept",
	"POST /api/fleet/mesh-offers/{id}/decline",
	"POST /api/fleet/peers/{id}/check/{domain}",
	"POST /api/fleet/peers/{id}/mesh-offer",
	"POST /api/group/join",
	"POST /api/group/phrase",
	"POST /api/group/phrase/show",
	"POST /api/group/probe",
	"POST /api/group/roles/requests/{id}",
	"POST /api/offsite/group-receivers/{id}/login",
	"POST /api/pull/sources",
	"POST /api/pull/sources/{id}/test",
	"POST /api/receiver/repos",
	"POST /api/receiver/repos/{id}/check",
	"POST /api/receiver/server",
	"POST /api/receiver/server/check",
	"POST /api/zfs/receive/requests/{id}",
	"POST /api/zfs/receive/{slot}/members/{member}/abort",
	"POST /api/zfs/replica/servers",
	"POST /api/zfs/replica/servers/test",
	"POST /api/zfs/replica/servers/{id}/forget-host-key",
	"POST /api/zfs/replica/servers/{id}/test",
	"PUT /api/fleet/peers/{id}",
	"PUT /api/group/address",
	"PUT /api/group/members/{id}/roles/{role}",
	"PUT /api/group/relay",
	"PUT /api/pull/sources/{id}",
	"PUT /api/receiver/repos/{id}",
	"PUT /api/zfs/receive/{slot}/members/{member}",
	// Storage locations, their connections and sign-ins, and where entries are placed.
	"DELETE /api/offsite/destinations/{id}",
	"DELETE /api/offsite/targets/{id}",
	"DELETE /api/repos/{id}",
	"DELETE /api/settings/primary-remote/{domain}",
	"GET /api/cloud",
	"GET /api/cloud/creds-sets",
	"GET /api/offsite/destinations/{id}/erasure",
	"GET /api/offsite/destinations/{id}/removal",
	"GET /api/offsite/targets/{id}/direct",
	"GET /api/offsite/targets/{id}/move",
	"GET /api/offsite/{domain}/deploy-snippet",
	"GET /api/placement/default/{domain}/apply",
	"GET /api/placement/default/{domain}/confirm",
	"GET /api/placement/defaults",
	"GET /api/placement/new-target-preview",
	"GET /api/placement/options",
	"GET /api/rclone",
	"GET /api/settings/primary-remote/{domain}",
	"PATCH /api/repos/{id}",
	"POST /api/cloud",
	"POST /api/cloud/creds-sets",
	"POST /api/encryption/detect",
	"POST /api/offsite/destinations",
	"POST /api/offsite/destinations/{id}/adopt/{target}",
	"POST /api/offsite/destinations/{id}/domains/{domain}",
	"POST /api/offsite/destinations/{id}/erasure",
	"POST /api/offsite/destinations/{id}/primary/{domain}",
	"POST /api/offsite/destinations/{id}/removal",
	"POST /api/offsite/drafts/check",
	"POST /api/offsite/drafts/folders",
	"POST /api/offsite/drafts/mkdir",
	"POST /api/offsite/rclone-remote",
	"POST /api/offsite/targets",
	"POST /api/offsite/targets/{id}/direct/test",
	"POST /api/offsite/targets/{id}/move",
	"POST /api/offsite/targets/{id}/tamper-test",
	"POST /api/offsite/targets/{id}/test",
	"POST /api/offsite/{domain}/tamper-test",
	"POST /api/offsite/{domain}/test",
	"POST /api/placement/default/{domain}/apply",
	"POST /api/placement/default/{domain}/confirm",
	"POST /api/placement/default/{domain}/preview",
	"POST /api/placement/exclude",
	"POST /api/rclone",
	"POST /api/repos",
	"POST /api/repos/{id}/connect",
	"POST /api/settings/primary-remote/{domain}/tamper-test",
	"POST /api/settings/primary-remote/{domain}/test",
	"PUT /api/offsite/destinations/{id}",
	"PUT /api/offsite/destinations/{id}/connection",
	"PUT /api/offsite/targets/{id}",
	"PUT /api/placement/default/{domain}",
	"PUT /api/settings/primary-remote/{domain}",
	// Settings, notifications, integrations, discovery and the settings backup.
	"DELETE /api/anomalies/items/{targetId}/expectations/{family}",
	"GET /api/config/snapshots",
	"GET /api/dashboard-plugin",
	"GET /api/homeassistant",
	"GET /api/mdns",
	"GET /api/notify",
	"GET /api/settings/export",
	"GET /api/settings/idle",
	"GET /api/settings/streaming",
	"GET /api/vm/ssh",
	"POST /api/anomalies/expected",
	"POST /api/config/restore",
	"POST /api/dashboard-plugin/install",
	"POST /api/dashboard-plugin/remove",
	"POST /api/discover",
	"POST /api/files/discover",
	"POST /api/import/appdata-backup",
	"POST /api/import/appdata-backup/scan",
	"POST /api/notify",
	"POST /api/notify/test",
	"POST /api/settings/import",
	"POST /api/spike",
	"POST /api/update-check",
	"POST /api/vm/ssh/test",
	"POST /api/vms/discover",
	"POST /api/zfs/check",
	"POST /api/zfs/discover",
	"PUT /api/anomalies/items/{targetId}/prefs",
	"PUT /api/homeassistant",
	"PUT /api/mdns",
	"PUT /api/settings",
	"PUT /api/settings/idle",
	"PUT /api/settings/streaming",
	// Downloads and exports of what is backed up, and the diagnostics bundle.
	"GET /api/containers/{name}/dbdumps/{id}/download",
	"GET /api/diagnostics",
	"GET /api/flash/download",
	"POST /api/containers/{name}/dbdumps/{id}/save",
	"POST /api/containers/{name}/export",
	"POST /api/items/{domain}/{name}/export",
	"POST /api/items/{domain}/{name}/export/preview",
	"POST /api/vms/{name}/export",
	// Changing, deleting and pruning entries and their backups.
	"DELETE /api/containers/{name}",
	"DELETE /api/containers/{name}/alias/{old}",
	"DELETE /api/containers/{name}/backups",
	"DELETE /api/files/sets/{id}",
	"DELETE /api/files/sets/{id}/backups",
	"DELETE /api/items/{domain}/{name}/offsite/{target}/removal",
	"DELETE /api/items/{domain}/{name}/timeline/{key}",
	"DELETE /api/snapshots/{domain}/{id}",
	"DELETE /api/vms/{name}",
	"DELETE /api/vms/{name}/alias/{old}",
	"DELETE /api/vms/{name}/backups",
	"DELETE /api/zfs/datasets/{id}",
	"DELETE /api/zfs/datasets/{id}/backups",
	"DELETE /api/zfs/datasets/{id}/safety-snapshots",
	"GET /api/containers/{name}/excludes/suggest",
	"GET /api/files/sets/preset",
	"GET /api/items/{domain}/{name}/offsite/{target}/removal",
	"GET /api/items/{domain}/{name}/timeline/{key}/delete",
	"GET /api/zfs/datasets/{id}/replica",
	"PATCH /api/containers/{name}",
	"PATCH /api/files/sets/{id}",
	"PATCH /api/vms/{name}",
	"PATCH /api/zfs/datasets/{id}",
	"PATCH /api/zfs/datasets/{id}/replica",
	"POST /api/browse/mkdir",
	"POST /api/checks/probe/{id}",
	"POST /api/checks/starttest/{id}",
	"POST /api/containers/schedule-include",
	"POST /api/containers/{name}/excludes/preview",
	"POST /api/containers/{name}/tag",
	"POST /api/containers/{name}/takeover",
	"POST /api/files/sets",
	"POST /api/items/{domain}/{name}/placement/preview",
	"POST /api/items/{domain}/{name}/tag",
	"POST /api/prune/{domain}",
	"POST /api/unlock/{domain}",
	"POST /api/vms/schedule-include",
	"POST /api/vms/{name}/takeover",
	"POST /api/zfs/datasets",
	"POST /api/zfs/datasets/{id}/probe",
	"POST /api/zfs/datasets/{id}/sweep",
	"POST /api/zfs/excludes/preview",
	"PUT /api/containers/backup-order",
	"PUT /api/items/{kind}/{key}/declined",
	"PUT /api/vms/backup-order",
	// Acknowledging an anomaly, which lets the backups held for it be pruned.
	"POST /api/anomalies/acknowledge",
	// Restores that replace what is there without a backup first, and a
	// plugin written back to the flash drive, which runs it as root at boot.
	"POST /api/containers/{name}/dbdumps/{id}/import",
	"POST /api/flash/plugins/restore",
	"POST /api/stacks/{project}/restore",
}

// sessionRoutes are the registered routes that need a session.
func sessionRoutes(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, r := range registeredRoutes(t) {
		path := r
		if _, p, ok := strings.Cut(r, " "); ok {
			path = p
		}
		if !authGatePublicPath(path) {
			out = append(out, r)
		}
	}
	return out
}

func TestEveryRouteIsDecidedForAPairedSession(t *testing.T) {
	routes := sessionRoutes(t)
	for _, r := range routes {
		allowed, refused := pairedRoutes[r], slices.Contains(pairedRefused, r)
		switch {
		case allowed && refused:
			t.Errorf("%s is both allowed and refused to a paired phone", r)
		case !allowed && !refused:
			t.Errorf("%s is neither allowed nor refused to a paired phone: add it to pairedRoutes or to pairedRefused", r)
		}
	}
	for r := range pairedRoutes {
		if !slices.Contains(routes, r) {
			t.Errorf("pairedRoutes lists %s, which is no route", r)
		}
	}
	for _, r := range pairedRefused {
		if !slices.Contains(routes, r) {
			t.Errorf("pairedRefused lists %s, which is no route", r)
		}
	}
}

// requestFor is a request that matches pattern, with every wildcard filled.
func requestFor(pattern string) *http.Request {
	method, path, _ := strings.Cut(pattern, " ")
	path = regexp.MustCompile(`\{[^}]+\}`).ReplaceAllString(path, "abc")
	r := httptest.NewRequest(method, path, strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	return r
}

func TestAPairedSessionReachesTheListedRoutesAndIsRefusedTheRest(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.authEnabled()
	paired := secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)
	full := secret.NewSessionToken(in.appKey, hash, epoch, time.Hour)

	// The gate is what decides, so every route answers from a handler that
	// does nothing.
	mux := http.NewServeMux()
	routes := sessionRoutes(t)
	for _, r := range routes {
		mux.HandleFunc(r, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	}
	gate := in.h.authGate(mux)
	for _, r := range routes {
		for _, session := range []string{paired, full} {
			req := requestFor(r)
			req.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: session}) //nolint:gosec // G124: a cookie on a test request
			w := httptest.NewRecorder()
			gate.ServeHTTP(w, req)
			want := http.StatusNoContent
			if session == paired && !pairedRoutes[r] {
				want = http.StatusForbidden
				if !strings.Contains(w.Body.String(), codeOwnLoginNeeded) {
					t.Errorf("%s refused a paired session without the code: %s", r, w.Body.String())
				}
			}
			if w.Code != want {
				t.Errorf("%s with the session of a %s answered %d, want %d", r, map[bool]string{true: "paired phone", false: "person"}[session == paired], w.Code, want)
			}
		}
	}
}

func TestAPairedSessionIsRefusedSomethingOfEveryKindItMayNotDo(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	hash, epoch, _ := in.h.authEnabled()
	in.session = secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)
	for _, route := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/api/auth/password", map[string]any{"password": "a password of my own"}},
		{http.MethodPost, "/api/tokens", map[string]any{"label": "mine"}},
		{http.MethodGet, "/api/recovery-kit", nil},
		{http.MethodPost, "/api/emergency/read", map[string]any{"key": strings.Repeat("e", 64)}},
		{http.MethodPost, "/api/group/phrase/show", map[string]any{"password": "x"}},
		{http.MethodPost, "/api/receiver/server", map[string]any{}},
		{http.MethodPost, "/api/offsite/targets", map[string]any{"domain": "files", "repo": "/somewhere"}},
		{http.MethodGet, "/api/cloud/creds-sets", nil},
		{http.MethodPut, "/api/settings", map[string]any{}},
		{http.MethodGet, "/api/notify", nil},
		{http.MethodGet, "/api/diagnostics", nil},
		{http.MethodPatch, "/api/containers/nginx", map[string]any{"preHook": "touch /tmp/x"}},
		{http.MethodPost, "/api/prune/files", map[string]any{}},
	} {
		code, out := in.do(t, route.method, route.path, route.body)
		if code != http.StatusForbidden || out["code"] != codeOwnLoginNeeded {
			t.Errorf("%s %s with a paired session: %d %v", route.method, route.path, code, out)
		}
	}
	if code, out := in.do(t, http.MethodGet, "/api/status", nil); code != http.StatusOK || out["ok"] != true {
		t.Fatalf("the overview with a paired session: %d %v", code, out)
	}
	if code, _ := in.do(t, http.MethodGet, "/api/no-such-route", nil); code != http.StatusNotFound {
		t.Fatalf("a path no route matches answers %d to a paired session", code)
	}
}

// pairedReads sends every GET route on pairedRoutes through the router with
// the instance's session and returns each answer by route. A wildcard takes
// the value fill gives for the path, and a handler that panics on this bare
// instance answers nothing, which leaks nothing either.
func pairedReads(in *instance, fill func(path string) string) map[string]string {
	wildcard := regexp.MustCompile(`\{[^}]+\}`)
	out := map[string]string{}
	for pattern := range pairedRoutes {
		method, path, _ := strings.Cut(pattern, " ")
		if method != http.MethodGet {
			continue
		}
		path = wildcard.ReplaceAllString(path, fill(path))
		func() {
			defer func() { _ = recover() }()
			req := httptest.NewRequest(method, path, nil)
			req.AddCookie(&http.Cookie{Name: sessionCookieNameHTTP, Value: in.session}) //nolint:gosec // G124: a cookie on a test request
			w := httptest.NewRecorder()
			in.router.ServeHTTP(w, req)
			out[pattern] = w.Body.String()
		}()
	}
	return out
}

func TestAPairedSessionSeesNoLocationOrHookInWhatItReads(t *testing.T) {
	in := newInstance(t, "attic", strings.Repeat("b2", 32))
	const (
		withSignIn = "rest:https://user:s3cr'et@box:8000/files"
		withParams = "rclone::s3,access_key_id=AKIDLEAK,secret_access_key='LEAK,KEY':bucket/vms"
		hook       = "curl -fsS https://hc-ping.com/hooksecret"
	)
	if _, err := in.st.MutateSettings(func(s *store.Settings) error {
		s.FilesPath, s.ZFSPath, s.ContainersPath, s.VMsPath = withSignIn, withSignIn, withSignIn, withParams
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := in.st.CreateOffsiteTarget(store.OffsiteTarget{Domain: "files", Name: "box", Repo: withSignIn, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	named, err := in.st.CreateOffsiteTarget(store.OffsiteTarget{Name: "named", Repo: withParams, Role: "repo", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	set, err := in.st.CreateFileSet(store.FileSet{Name: "docs", Path: "/mnt/user/docs", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ds, err := in.st.CreateZFSDataset(store.ZFSDataset{Dataset: "tank/data", Enabled: true, HookContainer: "db", PreSnapshot: hook, PostSnapshot: hook})
	if err != nil {
		t.Fatal(err)
	}
	fill := func(path string) string {
		switch {
		case strings.HasPrefix(path, "/api/files/sets/"):
			return set.ID
		case strings.HasPrefix(path, "/api/zfs/datasets/"):
			return ds.ID
		case strings.HasPrefix(path, "/api/storage/locations/"):
			return named.ID
		case strings.HasPrefix(path, "/api/items/"), strings.HasPrefix(path, "/api/retention/"):
			return "files"
		}
		return "abc"
	}
	leaks := []string{"s3cr", "AKIDLEAK", "LEAK,KEY", "hooksecret"}
	seen := map[string]bool{}
	for _, body := range pairedReads(in, fill) {
		for _, leak := range leaks {
			seen[leak] = seen[leak] || strings.Contains(body, leak)
		}
	}
	for _, leak := range leaks {
		if !seen[leak] {
			t.Fatalf("the person's own session does not read %q anywhere, so this test proves nothing", leak)
		}
	}
	hash, epoch, _ := in.h.authEnabled()
	in.session = secret.NewPairedSessionToken(in.appKey, hash, epoch, time.Hour)
	for route, body := range pairedReads(in, fill) {
		for _, leak := range leaks {
			if strings.Contains(body, leak) {
				t.Errorf("%s answered a paired session with %q", route, leak)
			}
		}
	}
}
