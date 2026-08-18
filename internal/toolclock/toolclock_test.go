package toolclock

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestPermissionRequestSetsStart(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_000_000, 0)
	if err := Record(dir, "sess", EventPermissionRequest, "toolu_1", now); err != nil {
		t.Fatal(err)
	}
	m := Load(dir, "sess")
	e, ok := m["toolu_1"]
	if !ok {
		t.Fatal("expected entry for toolu_1")
	}
	if !e.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", e.StartedAt, now)
	}
	if !e.EndedAt.IsZero() {
		t.Errorf("EndedAt should be zero while running, got %v", e.EndedAt)
	}
}

func TestPostToolUseSetsEnd(t *testing.T) {
	dir := t.TempDir()
	start := time.Unix(1_000_000, 0)
	end := start.Add(12 * time.Second)
	if err := Record(dir, "sess", EventPermissionRequest, "toolu_1", start); err != nil {
		t.Fatal(err)
	}
	if err := Record(dir, "sess", EventPostToolUse, "toolu_1", end); err != nil {
		t.Fatal(err)
	}
	e := Load(dir, "sess")["toolu_1"]
	if !e.StartedAt.Equal(start) {
		t.Errorf("StartedAt = %v, want %v", e.StartedAt, start)
	}
	if !e.EndedAt.Equal(end) {
		t.Errorf("EndedAt = %v, want %v", e.EndedAt, end)
	}
}

func TestPostToolUseFailureCounts(t *testing.T) {
	dir := t.TempDir()
	start := time.Unix(1_000_000, 0)
	end := start.Add(3 * time.Second)
	_ = Record(dir, "sess", EventPermissionRequest, "toolu_1", start)
	if err := Record(dir, "sess", EventPostToolUseFailure, "toolu_1", end); err != nil {
		t.Fatal(err)
	}
	if e := Load(dir, "sess")["toolu_1"]; !e.EndedAt.Equal(end) {
		t.Errorf("PostToolUseFailure should set EndedAt, got %v", e.EndedAt)
	}
}

func TestEndBackfillsStartWhenMissing(t *testing.T) {
	// A tool that completes without our ever seeing its PermissionRequest must
	// still get a StartedAt, so the statusline renders it as done rather than
	// stranding it as a perpetual hourglass.
	dir := t.TempDir()
	end := time.Unix(1_000_000, 0)
	if err := Record(dir, "sess", EventPostToolUse, "toolu_orphan", end); err != nil {
		t.Fatal(err)
	}
	e := Load(dir, "sess")["toolu_orphan"]
	if e.StartedAt.IsZero() {
		t.Error("StartedAt should be backfilled to EndedAt when start was never seen")
	}
	if !e.StartedAt.Equal(end) {
		t.Errorf("backfilled StartedAt = %v, want %v", e.StartedAt, end)
	}
}

func TestUnknownEventAndEmptyIDsAreNoOps(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_000_000, 0)
	if err := Record(dir, "sess", "PreToolUse", "toolu_1", now); err != nil {
		t.Fatal(err)
	}
	if len(Load(dir, "sess")) != 0 {
		t.Error("PreToolUse must not create an entry (we start on PermissionRequest only)")
	}
	if err := Record(dir, "sess", EventPermissionRequest, "", now); err != nil {
		t.Fatal(err)
	}
	if err := Record(dir, "", EventPermissionRequest, "toolu_1", now); err != nil {
		t.Fatal(err)
	}
	if len(Load(dir, "sess")) != 0 {
		t.Error("empty id/session must be a no-op")
	}
}

func TestPrunesOldCompletedEntries(t *testing.T) {
	dir := t.TempDir()
	base := time.Unix(1_000_000, 0)
	// An old completed tool, ended well beyond retention.
	_ = Record(dir, "sess", EventPermissionRequest, "old", base)
	_ = Record(dir, "sess", EventPostToolUse, "old", base.Add(time.Second))
	// A fresh event far in the future triggers the prune pass.
	future := base.Add(retention + time.Hour)
	_ = Record(dir, "sess", EventPermissionRequest, "fresh", future)
	m := Load(dir, "sess")
	if _, ok := m["old"]; ok {
		t.Error("old completed entry should have been pruned")
	}
	if _, ok := m["fresh"]; !ok {
		t.Error("fresh entry should remain")
	}
}

func TestRunningEntryNotPrunedByRetention(t *testing.T) {
	// A still-running tool (no EndedAt) must survive even if it started long
	// ago — a long-running agent can run for many minutes.
	dir := t.TempDir()
	base := time.Unix(1_000_000, 0)
	_ = Record(dir, "sess", EventPermissionRequest, "longrun", base)
	_ = Record(dir, "sess", EventPermissionRequest, "other", base.Add(retention+time.Hour))
	if _, ok := Load(dir, "sess")["longrun"]; !ok {
		t.Error("a running (un-ended) entry should not be pruned by retention")
	}
}

func TestLoadMissingSessionIsEmpty(t *testing.T) {
	dir := t.TempDir()
	if m := Load(dir, "nope"); len(m) != 0 {
		t.Errorf("missing sidecar should load empty, got %v", m)
	}
}

func TestConcurrentRecordsKeepAllKeys(t *testing.T) {
	// Parallel tools fire their hooks at the same instant; the flock'd
	// read-modify-write must not drop keys. (This is the whole reason the
	// writer locks instead of last-writer-wins on the file.)
	dir := t.TempDir()
	base := time.Unix(1_000_000, 0)
	const n = 40
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("toolu_%d", i)
			if err := Record(dir, "sess", EventPermissionRequest, id, base.Add(time.Duration(i)*time.Millisecond)); err != nil {
				t.Errorf("record %s: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	if got := len(Load(dir, "sess")); got != n {
		t.Errorf("expected %d entries after concurrent writes, got %d", n, got)
	}
}

func TestSessionIDSanitizedFromPathEscape(t *testing.T) {
	dir := t.TempDir()
	now := time.Unix(1_000_000, 0)
	// A session id with path separators must not write outside the cache dir;
	// it's sanitized to a flat filename, so the record round-trips under the
	// same sanitized key.
	evil := "../../etc/sess"
	if err := Record(dir, evil, EventPermissionRequest, "toolu_1", now); err != nil {
		t.Fatal(err)
	}
	if _, ok := Load(dir, evil)["toolu_1"]; !ok {
		t.Error("sanitized session id should round-trip")
	}
}
