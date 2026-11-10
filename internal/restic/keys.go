package restic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Key is one of a repository's keys as `restic key list --json` names it.
// Every key opens the same master key; HostName and UserName only say whom a
// key was added for.
type Key struct {
	ID       string `json:"id"`
	Current  bool   `json:"current"`
	UserName string `json:"userName"`
	HostName string `json:"hostName"`
	Created  string `json:"created"`
}

// KeyAddArgs returns the argv slice for `restic key add`, which reads the new
// password from newPasswordFile so it never shows in the process list.
func KeyAddArgs(repo string, m Mode, newPasswordFile, host, user string) []string {
	args := repoFlag(repo, m)
	args = append(args, retryLockFlags()...)
	return append(args, "key", "add", "--new-password-file", newPasswordFile, "--host", host, "--user", user)
}

// KeyListArgs returns the argv slice for `restic key list --json`.
func KeyListArgs(repo string, m Mode) []string {
	args := repoFlag(repo, m)
	return append(args, "key", "list", "--no-lock", "--json")
}

// KeyRemoveArgs returns the argv slice for `restic key remove`, which locks
// the repository exclusively.
func KeyRemoveArgs(repo string, m Mode, id string) []string {
	args := repoFlag(repo, m)
	args = append(args, retryLockFlags()...)
	return append(args, "key", "remove", "--", id)
}

// AddKey gives the repository a key for password, opened with the password m
// carries, and labels it with host and user.
func (r Restic) AddKey(ctx context.Context, repo string, m Mode, password, host, user string) error {
	dir, err := os.MkdirTemp("", "bombvault-key-")
	if err != nil {
		return fmt.Errorf("restic key add: %w", err)
	}
	defer os.RemoveAll(dir) //nolint:errcheck // a temporary folder of this process
	file := filepath.Join(dir, "password")
	if err := os.WriteFile(file, []byte(password), 0o600); err != nil {
		return fmt.Errorf("restic key add: %w", err)
	}
	_, err = r.run(ctx, KeyAddArgs(repo, m, file, host, user), m)
	return err
}

// Keys lists the repository's keys. The one m's password opens is Current.
func (r Restic) Keys(ctx context.Context, repo string, m Mode) ([]Key, error) {
	out, err := r.run(ctx, KeyListArgs(repo, m), m)
	if err != nil {
		return nil, err
	}
	var keys []Key
	if err := json.Unmarshal(out, &keys); err != nil {
		return nil, fmt.Errorf("restic key list: %w", err)
	}
	return keys, nil
}

// RemoveKey takes the key id out of the repository. restic refuses to remove
// the key m's password opens.
func (r Restic) RemoveKey(ctx context.Context, repo string, m Mode, id string) error {
	_, err := r.run(ctx, KeyRemoveArgs(repo, m, id), m)
	return err
}

// RepoID returns the id restic gave the repository when it was created, the
// same for every key that opens it.
func (r Restic) RepoID(ctx context.Context, repo string, m Mode) (string, error) {
	out, err := r.run(ctx, CatConfigArgs(repo, m), m)
	if err != nil {
		return "", err
	}
	return repoIDOf(out)
}

func repoIDOf(out []byte) (string, error) {
	var config struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &config); err != nil {
		return "", fmt.Errorf("read the repository config: %w", err)
	}
	if config.ID == "" {
		return "", errors.New("the repository config names no id")
	}
	return config.ID, nil
}
