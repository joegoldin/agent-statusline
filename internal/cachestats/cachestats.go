// Package cachestats reads the prompt-cache accounting the pi cache-optimizer
// extension persists next to pi's own state. The extension is the only writer;
// this is a read-only, best-effort consumer, so every failure is returned to
// the caller and ends with the widget hidden rather than with zeros on screen.
package cachestats

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// FileName is the sidecar the cache-optimizer extension writes.
const FileName = "pi-cache-optimizer-stats.json"

// Totals is one model's running cache accounting. The file also carries
// per-session totals and a legacy family map; neither is read here, because the
// producing extension's footer defaults to the "total" scope and that is the
// figure this statusline is replacing.
type Totals struct {
	Day                   string `json:"day"`
	TotalRequests         int    `json:"totalRequests"`
	HitRequests           int    `json:"hitRequests"`
	CachedInputTokens     int    `json:"cachedInputTokens"`
	CacheWriteInputTokens int    `json:"cacheWriteInputTokens"`
	TotalInputTokens      int    `json:"totalInputTokens"`
}

// Stats is the slice of the file this statusline needs.
type Stats struct {
	TotalsByModel map[string]Totals `json:"totalsByModel"`
}

// Load reads and parses path. A missing or malformed file returns (nil, err):
// the caller has no use for a half-populated Stats, and the error is only ever
// destined for the debug log.
func Load(path string) (*Stats, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Stats
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Lookup returns the totals recorded for one model, keyed "provider/id" exactly
// as the producing extension writes it. There is deliberately no bare-id
// fallback: two providers can serve the same model id, and rendering another
// provider's hit rate is worse than rendering nothing.
func (s *Stats) Lookup(provider, id string) (Totals, bool) {
	if s == nil || len(s.TotalsByModel) == 0 || provider == "" || id == "" {
		return Totals{}, false
	}
	t, ok := s.TotalsByModel[provider+"/"+id]
	return t, ok
}

// Path returns the stats file's location. PI_CODING_AGENT_DIR is pi's own
// override for its state directory, so following it keeps the statusline
// pointed at whichever pi is actually running.
func Path() string {
	if d := os.Getenv("PI_CODING_AGENT_DIR"); d != "" {
		return filepath.Join(d, FileName)
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pi", "agent", FileName)
}
