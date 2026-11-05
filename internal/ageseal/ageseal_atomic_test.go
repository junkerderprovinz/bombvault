package ageseal

import (
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestAFailedEncryptionLeavesTheFileThatWasThere(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "flash-latest.zip.age")
	if err := os.WriteFile(dst, []byte("the last good one"), 0o600); err != nil {
		t.Fatal(err)
	}

	// A folder opens and fails on the first read, as a disk that gives up does.
	if err := EncryptFile(dir, dst, []age.Recipient{id.Recipient()}); err == nil {
		t.Fatal("encrypting a folder went through")
	}

	got, err := os.ReadFile(dst) //nolint:gosec // G304: a temp path of this test
	if err != nil {
		t.Fatalf("the file that was there is gone: %v", err)
	}
	if string(got) != "the last good one" {
		t.Fatalf("the file that was there now holds %q", got)
	}
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("the failed write left its temp file behind: %v", err)
	}
}

func TestAnEncryptionReplacesTheFileInOneStep(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "plain"), filepath.Join(dir, "sealed.age")
	if err := os.WriteFile(src, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EncryptFile(src, dst, []age.Recipient{id.Recipient()}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(dst) //nolint:gosec // G304: a temp path of this test
	if err != nil || string(got) == "old" {
		t.Fatalf("the sealed file reads %q, %v", got, err)
	}
	if _, err := os.Stat(dst + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("the temp file stayed: %v", err)
	}
}
