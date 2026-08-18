package compaction

import (
	"os"
	"path/filepath"
	"testing"
)

const win = 200_000

func TestTrackFirstObservation(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	n, err := s.Track("session-a", 47.0, win)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("first observation should be 0, got %d", n)
	}
}

func TestTrackIncrementsOnLargeDrop(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	_, _ = s.Track("s", 90.0, win)
	n, _ := s.Track("s", 10.0, win) // 80pp drop, well above 30pp threshold
	if n != 1 {
		t.Errorf("count after drop = %d, want 1", n)
	}
	_, _ = s.Track("s", 70.0, win)
	if n, _ := s.Track("s", 5.0, win); n != 2 {
		t.Errorf("count after second drop = %d, want 2", n)
	}
}

func TestTrackIgnoresSmallDrops(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	_, _ = s.Track("s", 50.0, win)
	// 25pp drop is below the 30pp threshold — cache fluctuation, not compaction
	n, _ := s.Track("s", 25.0, win)
	if n != 0 {
		t.Errorf("small drop counted: %d", n)
	}
}

func TestTrackIgnoresModelSwitch(t *testing.T) {
	dir := t.TempDir()
	s := &Store{Dir: dir}
	// Established at 80% on a 200k window
	_, _ = s.Track("s", 80.0, 200_000)
	// User switches to a 1M-context model — same absolute tokens now sit at
	// roughly 16% of the new window. That's a 64pp drop but is NOT a /compact.
	n, _ := s.Track("s", 16.0, 1_000_000)
	if n != 0 {
		t.Errorf("model switch counted as compaction: %d", n)
	}
}

func TestStorePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	s1 := &Store{Dir: dir}
	_, _ = s1.Track("s", 90.0, win)
	_, _ = s1.Track("s", 5.0, win) // 85pp drop → +1

	s2 := &Store{Dir: dir}
	// Continuing the same session, no new drop: counter stays at 1
	n, _ := s2.Track("s", 8.0, win)
	if n != 1 {
		t.Errorf("after reload, count = %d, want 1", n)
	}

	stored := filepath.Join(dir, "s.json")
	if _, err := os.ReadFile(stored); err != nil {
		t.Errorf("expected store file to exist: %v", err)
	}
}
