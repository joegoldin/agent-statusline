// Package compaction maintains a per-session counter of /compact resets,
// detected as drops greater than dropThresholdPct in context_window.used_percentage
// between consecutive observations within the same context window size.
package compaction

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// dropThresholdPct — a real /compact typically drops 80%+. The conservative
// threshold avoids false positives from cache fluctuations and similar
// percentage noise.
const dropThresholdPct = 30.0

type Store struct {
	Dir string
}

type sessionState struct {
	LastPct        float64 `json:"last_pct"`
	LastWindowSize int     `json:"last_window_size"`
	Count          int     `json:"count"`
}

// Track records the current observation for sessionID and returns the
// running compaction count.
//
// A drop is counted as a compaction only when:
//
//   - There is a prior observation for this session (so resumes continue
//     to count from where they left off — same session_id keeps state),
//   - The context_window_size hasn't changed (switching to a larger model
//     causes the percentage to drop without any real compaction), and
//   - The percentage drop exceeds dropThresholdPct.
func (s *Store) Track(sessionID string, currentPct float64, windowSize int) (int, error) {
	path := s.pathFor(sessionID)
	state, existed, err := s.load(path)
	if err != nil {
		return 0, err
	}
	sameWindow := state.LastWindowSize == windowSize || state.LastWindowSize == 0
	if existed && sameWindow && state.LastPct-currentPct > dropThresholdPct {
		state.Count++
	}
	// Skip the disk write when nothing changed — at a 1s refresh this avoids
	// a write every tick during idle periods (the common case).
	unchanged := existed &&
		state.LastPct == currentPct &&
		state.LastWindowSize == windowSize
	if unchanged {
		return state.Count, nil
	}
	state.LastPct = currentPct
	state.LastWindowSize = windowSize
	if err := s.save(path, state); err != nil {
		return state.Count, err
	}
	return state.Count, nil
}

func (s *Store) pathFor(sessionID string) string {
	return filepath.Join(s.Dir, sessionID+".json")
}

func (s *Store) load(path string) (sessionState, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sessionState{}, false, nil
		}
		return sessionState{}, false, err
	}
	var st sessionState
	if err := json.Unmarshal(data, &st); err != nil {
		return sessionState{}, false, nil
	}
	return st, true, nil
}

func (s *Store) save(path string, state sessionState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
