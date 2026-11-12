//go:build windows

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"codexqueuestudiov3/internal"
)

// This test simulates a validly hashed but non-executable update.
// It verifies that the previous files are restored after launch fails.
func TestRollbackAfterInvalidExecutable(t *testing.T) {
	dir := t.TempDir()
	stage := filepath.Join(t.TempDir(), "stage")
	if e := os.MkdirAll(stage, 0700); e != nil {
		t.Fatal(e)
	}
	original := map[string][]byte{"CodexQueueStudio.exe": []byte("old-app"), "CodexQueueUpdater.exe": []byte("old-updater")}
	hashes := map[string]string{}
	for n, d := range original {
		if e := os.WriteFile(filepath.Join(dir, n), d, 0600); e != nil {
			t.Fatal(e)
		}
	}
	for _, n := range []string{"CodexQueueStudio.exe", "CodexQueueUpdater.exe"} {
		b := bytes.Repeat([]byte{byte(len(n))}, 100100)
		if e := os.WriteFile(filepath.Join(stage, n), b, 0600); e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(b)
		hashes[n] = hex.EncodeToString(sum[:])
	}
	b, _ := json.Marshal(internal.UpdateManifest{Version: "3.2.1", Files: hashes})
	if e := os.WriteFile(filepath.Join(stage, "update-manifest.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	err := install(99999999, filepath.Join(dir, "CodexQueueStudio.exe"), stage, nil)
	if err == nil {
		t.Fatal("invalid executable appeared to install")
	}
	for name, originalBytes := range original {
		now, e := os.ReadFile(filepath.Join(dir, name))
		if e != nil || !bytes.Equal(now, originalBytes) {
			t.Fatalf("rollback failed %s: %v", name, e)
		}
	}
}
