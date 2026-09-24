package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/restickey"
	"github.com/junkerderprovinz/bombvault/internal/secret"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// offsiteModeForTarget builds the restic mode for one off-site destination.
// It starts from the global mode (ModeFor sets Env and StorageClass from the
// shared CloudCreds), then applies the credential set the target names via
// CredsRef, overriding Env and, unless the target sets its own, StorageClass.
// A target with an empty CredsRef keeps the shared credentials. A target
// with an empty StorageClass, such as a backfilled N=1 target the SQL
// migration could not populate, keeps whichever class its credential set
// carries; overwriting it unconditionally would wipe the class for single
// off-site installs.
func (s *Service) offsiteModeForTarget(settings store.Settings, target store.OffsiteTarget) restic.Mode {
	mode := s.applyTargetCreds(s.ModeFor(settings), settings, target)
	if target.StorageClass != "" {
		mode.StorageClass = target.StorageClass
	}
	return mode
}

// applyTargetCreds overrides mode's Env (and storage class) with the
// credential set a target names; a target with an empty CredsRef is returned
// untouched.
//
// The off-site path (offsiteModeForTarget) and the primary path
// (primaryModeFor) share it because both ask the same row type the same
// question, and a second copy would let the two answers drift apart.
func (s *Service) applyTargetCreds(mode restic.Mode, settings store.Settings, target store.OffsiteTarget) restic.Mode {
	if strings.TrimSpace(target.CredsRef) == "" {
		return mode
	}
	c, err := s.decodeCloudFor(settings, target.CredsRef)
	if err != nil {
		log.Printf("api: target %s: cloud creds decode failed (ignoring, falling back to shared): %v", target.ID, err) //nolint:gosec // G706: target.ID is an opaque store-generated id
		return mode
	}
	mode.Env = cloudEnv(c)
	if c.S3StorageClass != "" {
		mode.StorageClass = c.S3StorageClass
	}
	return mode
}

// rcloneConfPath is where the decrypted rclone config is written for restic→rclone.
func (s *Service) rcloneConfPath() string { return filepath.Join(s.cfg.DataDir, "rclone.conf") }

// WriteRcloneConfFile (re)writes the on-disk rclone config from the encrypted
// value in settings, or removes it when empty. Called at startup so off-site
// repos work immediately after a restart.
func (s *Service) WriteRcloneConfFile() error {
	settings, err := s.store.GetSettings()
	if err != nil {
		return err
	}
	return s.writeRcloneFile(settings.RcloneConf)
}

// writeRcloneFile writes the decrypted rclone config (from its base64+AES-GCM
// stored form) to a 0600 file, or removes the file when the stored value is empty.
func (s *Service) writeRcloneFile(encB64 string) error {
	p := s.rcloneConfPath()
	if strings.TrimSpace(encB64) == "" {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("remove rclone conf: %w", err)
		}
		return nil
	}
	enc, err := base64.StdEncoding.DecodeString(encB64)
	if err != nil {
		return fmt.Errorf("decode rclone conf: %w", err)
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return fmt.Errorf("decrypt rclone conf: %w", err)
	}
	if err := os.WriteFile(p, plain, 0o600); err != nil {
		return fmt.Errorf("write rclone conf: %w", err)
	}
	// Guarantee 0600 even if the file existed with looser perms (WriteFile
	// only applies the mode on creation): it holds cleartext cloud credentials.
	if err := os.Chmod(p, 0o600); err != nil {
		return fmt.Errorf("chmod rclone conf: %w", err)
	}
	return nil
}

// SetRcloneConf encrypts + stores the rclone config and rewrites the on-disk
// file restic→rclone reads. An empty conf clears both. The stored DB value is
// AES-256-GCM-encrypted (APP_KEY); the on-disk file is 0600 in /config.
func (s *Service) SetRcloneConf(conf string) error {
	stored := ""
	if strings.TrimSpace(conf) != "" {
		enc, encErr := secret.Encrypt(s.cfg.AppKey, []byte(conf))
		if encErr != nil {
			return fmt.Errorf("encrypt rclone conf: %w", encErr)
		}
		stored = base64.StdEncoding.EncodeToString(enc)
	}
	if _, err := s.store.MutateSettings(func(settings *store.Settings) error {
		settings.RcloneConf = stored
		return nil
	}); err != nil {
		return err
	}
	return s.writeRcloneFile(stored)
}

// RcloneRemotes returns the configured rclone remote names (the [name]
// sections) for display, never the secrets themselves.
func (s *Service) RcloneRemotes() ([]string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(settings.RcloneConf) == "" {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.RcloneConf)
	if err != nil {
		return nil, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return nil, err
	}
	return parseRcloneRemotes(string(plain)), nil
}

// decodeRcloneConf returns the decrypted rclone config text stored in
// settings (an empty or blank rclone_conf yields "", no error). Unlike
// RcloneRemotes it keeps the full contents, for the recovery kit, which
// needs the remote secrets.
func (s *Service) decodeRcloneConf(settings store.Settings) (string, error) {
	if strings.TrimSpace(settings.RcloneConf) == "" {
		return "", nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.RcloneConf)
	if err != nil {
		return "", err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// parseRcloneRemotes extracts the [name] section headers from an rclone config.
func parseRcloneRemotes(conf string) []string {
	var out []string
	for _, line := range strings.Split(conf, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if name := strings.TrimSpace(line[1 : len(line)-1]); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

// CloudCreds holds the backend credentials restic reads from the environment for
// off-site repos. Stored AES-256-GCM-encrypted in settings.cloud_conf. The two
// secret fields (S3Secret, RESTPassword) are write-only over the API.
type CloudCreds struct {
	S3KeyID      string `json:"s3KeyId"`
	S3Secret     string `json:"s3Secret"`
	S3Region     string `json:"s3Region"`
	RESTUser     string `json:"restUser"`
	RESTPassword string `json:"restPassword"`
	// S3StorageClass is the S3 storage class for restic writes to a native s3:
	// off-site backend (empty = the provider default). Unlike the credential
	// fields it is not a secret (a class name), so handleGetCloud returns it.
	// It rides this same AES-256-GCM-encrypted cloud_conf blob, so it needs no
	// schema migration. It is validated against restic.AllowedStorageClasses
	// on save (SetCloudCreds), so only a restore-readable tier is ever stored
	// or emitted.
	S3StorageClass string `json:"s3StorageClass"`
}

// cloudEnv renders the credentials into the env vars restic expects (only the set
// ones), so they reach the restic process via Mode.Env and never via argv/logs.
func cloudEnv(c CloudCreds) []string {
	var env []string
	add := func(k, v string) {
		if v != "" {
			env = append(env, k+"="+v)
		}
	}
	add("AWS_ACCESS_KEY_ID", c.S3KeyID)
	add("AWS_SECRET_ACCESS_KEY", c.S3Secret)
	add("AWS_DEFAULT_REGION", c.S3Region)
	add("RESTIC_REST_USERNAME", c.RESTUser)
	add("RESTIC_REST_PASSWORD", c.RESTPassword)
	return env
}

// decodeCloud decrypts the stored cloud credentials from the given settings (an
// empty/blank cloud_conf yields a zero CloudCreds, no error).
func (s *Service) decodeCloud(settings store.Settings) (CloudCreds, error) {
	var c CloudCreds
	if strings.TrimSpace(settings.CloudConf) == "" {
		return c, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.CloudConf)
	if err != nil {
		return c, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(plain, &c); err != nil {
		return c, err
	}
	return c, nil
}

// CloudConfig returns the stored credentials. Callers that serve it to the
// UI must blank the secret fields (see handleGetCloud).
func (s *Service) CloudConfig() (CloudCreds, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return CloudCreds{}, err
	}
	return s.decodeCloud(settings)
}

// SetCloudCreds stores the credentials encrypted. A blank secret field keeps the
// previously stored secret (so the UI can edit non-secret fields without
// re-entering keys). A config with nothing set clears it.
func (s *Service) SetCloudCreds(c CloudCreds) error {
	// Normalize and validate the S3 storage class before anything is stored:
	// uppercase it, and reject a non-empty value that is not a whitelisted,
	// restore-readable tier (see restic.AllowedStorageClasses), so an archival
	// class that would break restic restore is never persisted. Empty = the
	// provider default.
	c.S3StorageClass = strings.ToUpper(strings.TrimSpace(c.S3StorageClass))
	if c.S3StorageClass != "" && !restic.StorageClassAllowed(c.S3StorageClass) {
		return fmt.Errorf("unsupported S3 storage class %q (allowed: %s)", c.S3StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
	}
	// The keep-prior merge below reads the currently stored secrets, so it has
	// to happen in the same transaction as the write. Against a snapshot taken
	// before the write it would re-encrypt a secret that a save landing in
	// between had already replaced, and the blank field would "keep" a value
	// that is no longer the stored one.
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		c := c
		// A fully blank request means "clear". Check it before the keep-prior
		// merge, otherwise the merge would re-fill the secrets and clearing would
		// be impossible once a secret had been stored.
		if (CloudCreds{}) == c {
			settings.CloudConf = ""
			return nil
		}
		// Otherwise keep a previously stored secret when its field is left blank, so
		// the non-secret fields can be edited without re-entering keys.
		prev, _ := s.decodeCloud(*settings)
		if c.S3Secret == "" {
			c.S3Secret = prev.S3Secret
		}
		if c.RESTPassword == "" {
			c.RESTPassword = prev.RESTPassword
		}
		blob, mErr := json.Marshal(c)
		if mErr != nil {
			return fmt.Errorf("marshal cloud conf: %w", mErr)
		}
		enc, eErr := secret.Encrypt(s.cfg.AppKey, blob)
		if eErr != nil {
			return fmt.Errorf("encrypt cloud conf: %w", eErr)
		}
		settings.CloudConf = base64.StdEncoding.EncodeToString(enc)
		return nil
	})
	return err
}

// CloudCredSet is one named, additional credential set an off-site target
// can opt into via OffsiteTarget.CredsRef (#141) instead of sharing the one
// CloudCreds set; an empty CredsRef keeps using CloudCreds. It embeds
// CloudCreds for the key, secret, region and storage-class fields, so
// cloudEnv and the storage-class validation in SetCloudCredSets stay shared
// with the single-set path.
type CloudCredSet struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// KeptFor is the id of the direct repository this set holds the values
	// of, from before a save changed them to ones that do not open it.
	KeptFor string `json:"keptFor,omitempty"`
	CloudCreds
}

// decodeCloudCredSets decrypts the stored additional credential sets from the
// given settings (an empty/blank cloud_cred_sets yields nil, no error).
func (s *Service) decodeCloudCredSets(settings store.Settings) ([]CloudCredSet, error) {
	if strings.TrimSpace(settings.CloudCredSets) == "" {
		return nil, nil
	}
	enc, err := base64.StdEncoding.DecodeString(settings.CloudCredSets)
	if err != nil {
		return nil, err
	}
	plain, err := secret.Decrypt(s.cfg.AppKey, enc)
	if err != nil {
		return nil, err
	}
	var sets []CloudCredSet
	if err := json.Unmarshal(plain, &sets); err != nil {
		return nil, err
	}
	return sets, nil
}

// CloudCredSets returns the additional named credential sets with every
// secret field blanked, for serving the list to the UI (the same
// blank-secrets contract as handleGetCloud). Callers that need the real
// secrets, such as restic env building, go through decodeCloudFor.
func (s *Service) CloudCredSets() ([]CloudCredSet, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return nil, err
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return nil, err
	}
	out := make([]CloudCredSet, len(sets))
	for i, set := range sets {
		out[i] = set
		out[i].S3Secret = ""
		out[i].RESTPassword = ""
	}
	return out, nil
}

// SetCloudCredSets replaces the whole list of additional named credential
// sets. Each set's secret fields follow the same keep-prior-if-blank rule as
// SetCloudCreds (matched by ID against the previously stored set), so the UI
// can rename a set or edit its non-secret fields without re-entering keys.
// KeptFor is carried over the same way, since the Settings page never sends
// it. A set with a blank Name or a duplicate ID is rejected: both would make
// CredsRef resolution ambiguous or the set unreachable from the UI.
func (s *Service) SetCloudCredSets(sets []CloudCredSet) error {
	// Same reasoning as SetCloudCreds: the keep-prior-if-blank merge reads the
	// sets stored right now, so it belongs in the same transaction as the write.
	// The incoming slice is copied rather than normalized in place, because the
	// caller's value is theirs and the merged one carries real secrets.
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		next := make([]CloudCredSet, len(sets))
		copy(next, sets)

		prev, _ := s.decodeCloudCredSets(*settings)
		prevByID := make(map[string]CloudCredSet, len(prev))
		for _, p := range prev {
			prevByID[p.ID] = p
		}
		seen := make(map[string]bool, len(next))
		for i := range next {
			next[i].Name = strings.TrimSpace(next[i].Name)
			if next[i].Name == "" {
				return fmt.Errorf("credential set name must not be empty")
			}
			if next[i].ID == "" {
				return fmt.Errorf("credential set %q: missing id", next[i].Name)
			}
			if seen[next[i].ID] {
				return fmt.Errorf("duplicate credential set id %q", next[i].ID)
			}
			seen[next[i].ID] = true
			next[i].S3StorageClass = strings.ToUpper(strings.TrimSpace(next[i].S3StorageClass))
			if next[i].S3StorageClass != "" && !restic.StorageClassAllowed(next[i].S3StorageClass) {
				return fmt.Errorf("credential set %q: unsupported S3 storage class %q (allowed: %s)", next[i].Name, next[i].S3StorageClass, strings.Join(restic.AllowedStorageClasses, ", "))
			}
			if old, ok := prevByID[next[i].ID]; ok {
				if next[i].S3Secret == "" {
					next[i].S3Secret = old.S3Secret
				}
				if next[i].RESTPassword == "" {
					next[i].RESTPassword = old.RESTPassword
				}
				if next[i].KeptFor == "" {
					next[i].KeptFor = old.KeptFor
				}
			}
		}
		enc, err := s.encodeCloudCredSets(next)
		if err != nil {
			return err
		}
		settings.CloudCredSets = enc
		return nil
	})
	return err
}

func (s *Service) encodeCloudCredSets(sets []CloudCredSet) (string, error) {
	if len(sets) == 0 {
		return "", nil
	}
	blob, err := json.Marshal(sets)
	if err != nil {
		return "", fmt.Errorf("marshal cloud cred sets: %w", err)
	}
	enc, err := secret.Encrypt(s.cfg.AppKey, blob)
	if err != nil {
		return "", fmt.Errorf("encrypt cloud cred sets: %w", err)
	}
	return base64.StdEncoding.EncodeToString(enc), nil
}

// editCloudCredSets applies edit to the stored credential sets, secrets
// included, inside one settings mutation. A list that comes back unchanged is
// not written, since encrypting it again would still change the row.
func (s *Service) editCloudCredSets(edit func([]CloudCredSet) []CloudCredSet) error {
	_, err := s.store.MutateSettings(func(settings *store.Settings) error {
		sets, err := s.decodeCloudCredSets(*settings)
		if err != nil {
			return fmt.Errorf("read the credential sets: %w", err)
		}
		next := edit(slices.Clone(sets))
		if slices.Equal(next, sets) {
			return nil
		}
		enc, err := s.encodeCloudCredSets(next)
		if err != nil {
			return err
		}
		settings.CloudCredSets = enc
		return nil
	})
	return err
}

// decodeCloudFor resolves the credentials an off-site target should use:
// the shared CloudCreds when credsRef is empty, or the matching named
// CloudCredSet otherwise. A credsRef that no longer resolves (the set was
// deleted, or storage drifted) falls back to the shared creds rather than
// failing the caller outright; restic then fails loudly on auth if that
// fallback has no usable credentials for this target's endpoint, which is
// a clearer signal than an opaque config error.
func (s *Service) decodeCloudFor(settings store.Settings, credsRef string) (CloudCreds, error) {
	if strings.TrimSpace(credsRef) == "" {
		return s.decodeCloud(settings)
	}
	sets, err := s.decodeCloudCredSets(settings)
	if err != nil {
		return CloudCreds{}, err
	}
	for _, set := range sets {
		if set.ID == credsRef {
			return set.CloudCreds, nil
		}
	}
	log.Printf("api: off-site target references unknown credential set %q, falling back to shared credentials", credsRef)
	return s.decodeCloud(settings)
}

// namedRepoItemNames lists the containers, VMs and folder sets pointed at one
// named repository (#204), as a single comma-separated line for the recovery
// kit. Built from the three item lists rather than a new store query, and
// tolerant of a read failure: the location is the part that must not be lost,
// the inventory is the help.
func (s *Service) namedRepoItemNames(id string) string {
	var out []string
	if tgs, err := s.store.ListTargets(); err == nil {
		for _, t := range tgs {
			if strings.TrimSpace(t.Repo) == id {
				out = append(out, "container "+t.ContainerName)
			}
		}
	}
	if vms, err := s.store.ListVMTargets(); err == nil {
		for _, v := range vms {
			if strings.TrimSpace(v.Repo) == id {
				out = append(out, "VM "+v.Name)
			}
		}
	}
	if sets, err := s.store.ListFileSets(); err == nil {
		for _, f := range sets {
			if strings.TrimSpace(f.Repo) == id {
				out = append(out, "folder set "+f.Name)
			}
		}
	}
	return strings.Join(out, ", ")
}

// recoveryRepo is one domain's resolved repo locations for the recovery kit.
type recoveryRepo struct {
	Domain  string
	Local   string
	Offsite string // "" when none configured
}

// RecoveryKit builds the plain-text/markdown recovery document the
// authenticated owner downloads to survive a loss of BombVault itself.
// With encryption on it contains the master APP_KEY and the APP_KEY-derived
// restic repository password the engine uses (restickey.Derive), the
// per-domain repo locations, and step-by-step manual `restic restore`
// instructions that need no BombVault container. With encryption off the
// repos use `--insecure-no-password`, so the kit's value is mainly the repo
// locations and the instructions.
//
// The document contains the master key, so it must never be logged and
// must be stored offline by the user (the handler streams it as an
// attachment only to the session-authenticated owner).
func (s *Service) RecoveryKit() (string, error) {
	settings, err := s.store.GetSettings()
	if err != nil {
		return "", fmt.Errorf("read settings: %w", err)
	}

	// Resolve each domain's local + off-site repo locations from the configured
	// settings (the same resolution the engine uses), so the kit names the real
	// places the data lives. A resolution failure for one domain leaves that line
	// blank rather than failing the whole kit.
	repos := make([]recoveryRepo, 0, 4)
	for _, d := range []string{"containers", "vms", "flash", "files"} {
		rr := recoveryRepo{Domain: d}
		if loc, rErr := s.repoFor(settings, d, "local"); rErr == nil {
			rr.Local = loc
		}
		if off := s.offsiteRepoFor(d, settings); off != "" {
			if loc, rErr := s.resolveRepo(off); rErr == nil {
				rr.Offsite = loc
			} else {
				rr.Offsite = off
			}
		}
		repos = append(repos, rr)
	}

	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	w("# BombVault encryption-key recovery kit\n\n")
	w("Generated: %s\n\n", time.Now().Format(time.RFC1123))
	w("> WARNING: this file is the master secret for your encrypted backups.\n")
	w("> It contains your APP_KEY and the derived restic repository password.\n")
	w("> Store it offline and securely (a password manager or printed copy in a safe).\n")
	w("> Anyone with this file can read and restore your backups.\n\n")

	w("## Encryption\n\n")
	if settings.EncryptionEnabled {
		password := restickey.Derive(s.cfg.AppKey)
		w("Status: ENABLED\n\n")
		w("APP_KEY (the master key; recreate the BombVault container with this exact value):\n\n")
		w("    %s\n\n", s.cfg.AppKey)
		w("restic repository password (derived from APP_KEY; use this with plain restic):\n\n")
		w("    %s\n\n", password)
	} else {
		w("Status: DISABLED\n\n")
		w("The repositories are created without a password (restic --insecure-no-password).\n")
		w("There is no key to lose; the value of this kit is the repository locations and\n")
		w("the restore instructions below.\n\n")
	}

	w("## Repository locations\n\n")
	w("Paths are inside the BombVault container, under the host data mount (%s).\n", s.cfg.HostMountRoot)
	w("On the host they live under your backup share; remote backends (rclone:/s3:/rest:/sftp:) are used as shown.\n\n")
	for _, rr := range repos {
		w("- %s (local): %s\n", rr.Domain, orNone(rr.Local))
		if rr.Offsite != "" {
			w("- %s (off-site): %s\n", rr.Domain, rr.Offsite)
		}
	}
	w("\n")
	w("Each line above is a separate restic repository. Point restic (or a tool like\n")
	w("backrest) at the specific per-domain path. The parent folder that holds them is\n")
	w("not itself a repository, and the off-site repo only has snapshots once off-site\n")
	w("replication has actually run. Add each domain repo on its own.\n\n")

	// Named repositories (#204): the locations individual containers, VMs and
	// folder sets were pointed at instead of their domain's own. Their
	// locations exist only in the database, so without this section a lost
	// /config would leave their intact data unfindable, while the domain
	// repositories above come from settings the user configured and can
	// re-derive. Each line names the items that were pointed at it, so the kit
	// answers both "where is it" and "what is in there".
	if named, nErr := s.store.ListNamedRepos(); nErr == nil && len(named) > 0 {
		w("## Named repositories (per-item)\n\n")
		w("These repositories hold the backups of individual containers, VMs or folder\n")
		w("sets that were pointed at them instead of their domain repository above. They\n")
		w("are ordinary restic repositories and use the same password as the rest.\n\n")
		for _, n := range named {
			loc := n.Repo
			if resolved, rErr := s.resolveRepo(n.Repo); rErr == nil {
				loc = resolved
			}
			w("- %s: %s\n", n.Name, loc)
			if items := s.namedRepoItemNames(n.ID); items != "" {
				w("  holds: %s\n", items)
			}
			if !n.Enabled {
				w("  (switched off at the time this kit was written)\n")
			}
		}
		w("\n")
	}

	// BombVault's own settings backup (the "config" self-backup domain). This
	// repo is the bootstrap seed a rebuilt box needs: restore it first to bring
	// BombVault's configuration back, then the data domains follow. It uses
	// the APP_KEY-derived restic password documented above, so no new secret
	// appears here. A resolution failure leaves the local line blank rather
	// than failing the kit; the off-site line prints only when one is
	// configured.
	w("## BombVault settings backup (config domain)\n\n")
	w("This repository holds BombVault's own settings. On a rebuilt box, restore it\n")
	w("first to bring BombVault's configuration back, then use the data repositories\n")
	w("above. It is the one location to write down so a fresh install can find itself.\n\n")
	configLocal := ""
	if loc, cErr := s.configRepoPath(settings); cErr == nil {
		configLocal = loc
	}
	w("- config (local): %s\n", orNone(configLocal))
	if settings.ConfigOffsite != "" {
		w("- config (off-site): %s\n", settings.ConfigOffsite)
	}
	w("\n")

	// Off-site and cloud credentials: the stored rest-server and S3 keys and
	// rclone config a user needs to reach a remote repository after losing
	// BombVault. These are secrets too, covered by the master-secret warning
	// above; like the APP_KEY they go only into this downloaded kit and are
	// never logged. Only the fields that are set are printed (as in cloudEnv),
	// so the section never shows an empty label.
	creds, _ := s.decodeCloud(settings)
	rcloneConf, _ := s.decodeRcloneConf(settings)
	hasREST := creds.RESTUser != "" || creds.RESTPassword != ""
	hasS3 := creds.S3KeyID != "" || creds.S3Secret != "" || creds.S3Region != ""
	hasRclone := strings.TrimSpace(rcloneConf) != ""

	w("## Repository credentials\n\n")
	if !hasREST && !hasS3 && !hasRclone {
		w("No off-site/cloud credentials are stored in BombVault.\n\n")
	} else {
		w("These are the stored off-site backend credentials: the same secrets restic\n")
		w("reads from its environment (or the rclone config) to reach a remote repository.\n")
		w("They are as sensitive as the APP_KEY above; keep them just as safe.\n\n")

		if hasREST {
			w("rest-server (restic REST backend). restic reads these from the environment:\n\n")
			if creds.RESTUser != "" {
				w("    RESTIC_REST_USERNAME=%s\n", creds.RESTUser)
			}
			if creds.RESTPassword != "" {
				w("    RESTIC_REST_PASSWORD=%s\n", creds.RESTPassword)
			}
			w("\n")
			w("Export these before running restic against a rest: repository. They can also\n")
			w("live inside the URL, e.g. rest:https://user:pass@host:8000/path.\n\n")
		}

		if hasS3 {
			w("S3-compatible backend. restic reads these from the environment:\n\n")
			if creds.S3KeyID != "" {
				w("    AWS_ACCESS_KEY_ID=%s\n", creds.S3KeyID)
			}
			if creds.S3Secret != "" {
				w("    AWS_SECRET_ACCESS_KEY=%s\n", creds.S3Secret)
			}
			if creds.S3Region != "" {
				w("    AWS_DEFAULT_REGION=%s\n", creds.S3Region)
			}
			w("\n")
			w("Export these before running restic against an s3: repository.\n\n")
		}

		if hasRclone {
			w("rclone config, which holds each remote's own secrets. Save it verbatim as\n")
			w("~/.config/rclone/rclone.conf, then use the repo as rclone:<remote>:<path>:\n\n")
			w("```\n%s\n```\n\n", strings.TrimRight(rcloneConf, "\n"))
		}
	}

	w("## Manual restore without BombVault\n\n")
	w("You can restore directly with the restic CLI, no BombVault container required.\n\n")
	w("1. Install restic (https://restic.net) on any machine that can reach the repository.\n")
	if settings.EncryptionEnabled {
		w("2. Set the repository password from this kit:\n\n")
		w("       export RESTIC_PASSWORD='%s'\n\n", restickey.Derive(s.cfg.AppKey))
	} else {
		w("2. The repositories have no password; pass --insecure-no-password to every\n")
		w("   restic command below (e.g. `restic -r <repo> --insecure-no-password snapshots`).\n\n")
	}
	w("3. List the snapshots in a repository (use a path or remote from the list above):\n\n")
	w("       restic -r <repo> snapshots\n\n")
	w("4. Restore a snapshot into a target directory (`restic restore`):\n\n")
	w("       restic -r <repo> restore <snapshot-id> --target <restore-dir>\n\n")
	w("Notes:\n")
	w("- For a local repo, point <repo> at the backup folder on disk (the path above is the\n")
	w("  container view; on the host it is your backup share, e.g. /mnt/user/<...>).\n")
	w("- For an rclone remote, configure rclone (~/.config/rclone/rclone.conf) and use the\n")
	w("  repo verbatim, e.g. `restic -r rclone:remote:bucket/path snapshots`.\n")
	w("- For an S3/B2/REST/SFTP remote, export the backend credentials restic expects\n")
	w("  (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY for S3, RESTIC_REST_USERNAME /\n")
	w("  RESTIC_REST_PASSWORD for a REST server) and use the repo verbatim.\n")

	return b.String(), nil
}

// orNone returns s, or "(not resolved)" when s is empty, so a blank repo line in
// the recovery kit reads clearly instead of trailing off.
func orNone(s string) string {
	if s == "" {
		return "(not resolved)"
	}
	return s
}
