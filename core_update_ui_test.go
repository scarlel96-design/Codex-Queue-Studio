package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codexqueuestudiov3/internal"
)

func updateTestZIP(t *testing.T) []byte {
	t.Helper()
	bins := map[string][]byte{"CodexQueueStudio.exe": bytes.Repeat([]byte("X"), 100500), "CodexQueueUpdater.exe": bytes.Repeat([]byte("Y"), 100501)}
	manifest := internal.UpdateManifest{Version: "3.2.1", Files: map[string]string{}}
	for n, data := range bins {
		sum := sha256.Sum256(data)
		manifest.Files[n] = hex.EncodeToString(sum[:])
	}
	b, _ := json.Marshal(manifest)
	bins["update-manifest.json"] = b
	var output bytes.Buffer
	z := zip.NewWriter(&output)
	for name, data := range bins {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
func TestDownloadUpdateProgressAndValidation(t *testing.T) {
	zipData := updateTestZIP(t)
	web := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(zipData) }))
	defer web.Close()
	hash := sha256.Sum256(zipData)
	c, err := NewCore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	steps := []int{}
	stage, err := c.DownloadUpdateProgress(context.Background(), UpdateInfo{URL: web.URL, Tag: "v3.2.1", Size: int64(len(zipData)), Digest: hex.EncodeToString(hash[:])}, func(_ string, pct int, _, _ int64) { steps = append(steps, pct) })
	if err != nil {
		t.Fatal(err)
	}
	if stage == "" || len(steps) < 4 || steps[0] != 0 || steps[len(steps)-1] != 92 {
		t.Fatalf("progress %v", steps)
	}
	for i := 1; i < len(steps); i++ {
		if steps[i] < steps[i-1] {
			t.Fatalf("progress regressed: %v", steps)
		}
	}
}
func TestReleaseNotesNoFabrication(t *testing.T) {
	if got := releaseNotes("  ## 개선\n실제 변경 사항  "); got != "## 개선\n실제 변경 사항" {
		t.Fatal(got)
	}
	if got := releaseNotes(""); !strings.Contains(got, "등록되지 않았") {
		t.Fatal(got)
	}
}
func TestInstallGuardActiveSendAndDueJob(t *testing.T) {
	c, e := NewCore(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	c.sendMu.Lock()
	if c.CanInstallUpdate() == nil {
		t.Fatal("ignored active send")
	}
	c.sendMu.Unlock()
	if c.CanInstallUpdate() != nil {
		t.Fatal("unexpected busy")
	}
	c.sessions = []internal.Session{{ID: "11111111-1111-1111-1111-111111111111", Title: "unit"}}
	if _, e = c.Schedule(c.sessions[0].ID, "unit", "hello", time.Now()); e != nil {
		t.Fatal(e)
	}
	if c.CanInstallUpdate() == nil {
		t.Fatal("did not protect imminent queued message")
	}
}
