package main

import (
	"codexqueuestudiov3/internal"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScheduleCancelAndArchiveGuard(t *testing.T) {
	c, err := NewCore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := internal.Session{ID: "11111111-1111-1111-1111-111111111111", Title: "테스트"}
	c.sessions = []internal.Session{s}
	when := time.Now().Add(time.Hour)
	id, err := c.Schedule(s.ID, s.Title, "Hello", when)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Pending()) != 1 || c.Pending()[0].State != "pending" {
		t.Fatal("not persisted")
	}
	if err = c.Cancel(id); err != nil {
		t.Fatal(err)
	}
	if c.Pending()[0].State != "canceled" {
		t.Fatal("cancel failed")
	}
	if err = c.Cancel(id); err == nil {
		t.Fatal("double cancellation must fail")
	}
	c.sessions[0].Archived = true
	if _, err = c.Schedule(s.ID, s.Title, "Hey", when); err == nil {
		t.Fatal("archived session accepted")
	}
}
func TestMissingEngineFailNoDuplicate(t *testing.T) {
	c, err := NewCore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := internal.Session{ID: "11111111-1111-1111-1111-111111111111", Title: "test"}
	c.sessions = []internal.Session{s}
	_, err = c.Schedule(s.ID, s.Title, "hello", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	c.SendDue()
	c.SendDue()
	jobs := c.Pending()
	if len(jobs) != 1 || jobs[0].State != "failed" {
		t.Fatalf("unexpected %+v", jobs)
	}
	// Failed tasks are not automatically replayed by the timer.
	if len(c.store.Snapshot().Events) != 3 {
		t.Fatalf("events %+v", c.store.Snapshot().Events)
	}
}
func TestUpdateSemverNoDowngrade(t *testing.T) {
	for _, c := range []struct {
		remote, current string
		yes             bool
	}{{"v3.0.1", "3.0.0", true}, {"v2.9.9", "3.0.0", false}, {"v3.0.0", "3.0.0", false}, {"v3.1.0", "3.0.9", true}, {"bad", "3.0.0", false}} {
		if v := newerTag(c.remote, c.current); v != c.yes {
			t.Errorf("%q %q -> %v", c.remote, c.current, v)
		}
	}
}
func TestExistingV2StoreMigrates(t *testing.T) {
	d := t.TempDir()
	json := `{"version":2,"settings":{"codexHome":"/tmp","theme":"dark"},"jobs":[],"events":[]}`
	if err := os.WriteFile(filepath.Join(d, "state.json"), []byte(json), 0600); err != nil {
		t.Fatal(err)
	}
	core, err := NewCore(d)
	if err != nil {
		t.Fatal(err)
	}
	if core.store.Snapshot().Settings.Theme != "dark" {
		t.Fatal("settings lost")
	}
}
