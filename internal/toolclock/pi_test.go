package toolclock

import (
	"testing"
	"time"
)

func TestRecordStartAndEndCarryName(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Unix(1748260800, 0)

	if err := RecordStart(dir, "s1", "call-1", "bash", "go test ./...", t0); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}
	e := Load(dir, "s1")["call-1"]
	if e.Name != "bash" {
		t.Errorf("Name = %q, want bash", e.Name)
	}
	if e.Target != "go test ./..." {
		t.Errorf("Target = %q", e.Target)
	}
	if !e.StartedAt.Equal(t0) {
		t.Errorf("StartedAt = %v, want %v", e.StartedAt, t0)
	}
	if !e.EndedAt.IsZero() {
		t.Error("EndedAt set before the end event")
	}

	t1 := t0.Add(3 * time.Second)
	if err := RecordEnd(dir, "s1", "call-1", t1); err != nil {
		t.Fatalf("RecordEnd: %v", err)
	}
	e = Load(dir, "s1")["call-1"]
	if !e.EndedAt.Equal(t1) {
		t.Errorf("EndedAt = %v, want %v", e.EndedAt, t1)
	}
	if e.Name != "bash" {
		t.Errorf("Name lost across end event: %q", e.Name)
	}
}

func TestRecordEndBackfillsStart(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Unix(1748260800, 0)
	if err := RecordEnd(dir, "s1", "orphan", t0); err != nil {
		t.Fatalf("RecordEnd: %v", err)
	}
	e := Load(dir, "s1")["orphan"]
	if e.StartedAt.IsZero() {
		t.Error("StartedAt not backfilled; the row would render as a perpetual hourglass")
	}
}

func TestRecordStartIgnoresEmptyIdentifiers(t *testing.T) {
	dir := t.TempDir()
	if err := RecordStart(dir, "", "c", "bash", "", time.Now()); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}
	if err := RecordStart(dir, "s", "", "bash", "", time.Now()); err != nil {
		t.Fatalf("RecordStart: %v", err)
	}
	if n := len(Load(dir, "s")); n != 0 {
		t.Errorf("wrote %d entries for empty identifiers, want 0", n)
	}
}

func TestClaudeRecordStillWorksAndLeavesNameEmpty(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Unix(1748260800, 0)
	if err := Record(dir, "s1", EventPermissionRequest, "tu-1", t0); err != nil {
		t.Fatalf("Record: %v", err)
	}
	e := Load(dir, "s1")["tu-1"]
	if !e.StartedAt.Equal(t0) {
		t.Errorf("StartedAt = %v", e.StartedAt)
	}
	if e.Name != "" {
		t.Errorf("Name = %q; Claude sources names from the transcript, not the sidecar", e.Name)
	}
}
