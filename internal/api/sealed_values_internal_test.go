package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/store"
)

const (
	// keptWithLogin is a sealed value of the login, which a settings restore
	// of another install replaces with this server's own.
	keptWithLogin = "login"
	// keptOutside is a sealed value that never reaches the settings database.
	keptOutside = "outside the database"
)

// sealSites names, for every function that seals a value with secret.Encrypt,
// the column the value is kept in. A settings restore of another install
// seals every one of those columns again under this server's APP_KEY, and it
// finds them by their name: a BLOB column ends in _enc, a settings column of
// base64 text is listed in store.SealedSettingsText.
var sealSites = map[string]string{
	"sealDefinition":         keptOutside,
	"depositSeed":            "key_deposit_signer.seed_enc",
	"depositRecipient":       "key_deposit_identity.recipient_enc",
	"newDepositIdentity":     "key_deposit_identity.recipient_enc",
	"takeOverStagedSettings": keptOutside,
	"finishTakeover":         "backup_key.key_enc",
	"AdoptEmergencyKey":      "backup_key.key_enc",
	"storeGroupSecret":       "group_state.secret_enc",
	"keepFormerGroup":        "group_state.former_peer_auth_enc",
	"pairedPassword":         "received_repos.restic_password_enc",
	"ConvertLegacyAppKeys":   "pull_sources.restic_password_enc",
	"encryptTOTPSecret":      keptWithLogin,
	"sealBrokerPassword":     "mqtt_settings.password_enc",
	"handlePeerMeshOffer":    "mesh_offers.rest_password_enc",
	"addHtpasswdUser":        "receiver_logins.password_enc",
	"EncodeRegistryAuths":    "settings.registry_auths",
	"storeRcloneConf":        "settings.rclone_conf",
	"SetNotifyConfig":        "settings.notify_conf",
	"sealSharedBlob":         "replaced_sign_ins.cloud_conf_enc",
	"encodeCloudCredSets":    "settings.cloud_cred_sets",
	"zfsReceiveAllow":        "zfs_receive_slots.token_enc",
	"askZFSReceive":          "zfs_datasets.replica_peer_token_enc",
}

// sealingFunctions returns the functions of the module, tests left out, that
// call secret.Encrypt.
func sealingFunctions(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	found := map[string]bool{}
	err := filepath.WalkDir(filepath.Join("..", ".."), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "web", "node_modules", "testdata", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		// The package is found by its path, so that an import under another
		// name does not hide a call.
		secretName := ""
		for _, imp := range file.Imports {
			if strings.HasSuffix(strings.Trim(imp.Path.Value, `"`), "/internal/secret") {
				secretName = "secret"
				if imp.Name != nil {
					secretName = imp.Name.Name
				}
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || secretName == "" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == secretName && sel.Sel.Name == "Encrypt" {
					found[fn.Name.Name] = true
				}
				return true
			})
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(found))
	for name := range found {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func TestEverySealedValueIsOneASettingsRestoreCarriesOver(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	columns, err := store.SealedColumns(db)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, c := range columns {
		known[c.Table+"."+c.Column] = true
	}

	sealing := sealingFunctions(t)
	for _, fn := range sealing {
		kept, listed := sealSites[fn]
		switch {
		case !listed:
			t.Errorf("%s seals a value with secret.Encrypt and sealSites does not say where it is kept", fn)
		case kept == keptWithLogin, kept == keptOutside:
		case !known[kept]:
			t.Errorf("%s keeps its sealed value in %s, which a settings restore does not seal again: name a BLOB column *_enc, or add the settings column to store.SealedSettingsText", fn, kept)
		}
	}
	for fn := range sealSites {
		if !slices.Contains(sealing, fn) {
			t.Errorf("sealSites lists %s, which seals nothing", fn)
		}
	}
}
