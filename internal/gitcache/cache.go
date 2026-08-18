package gitcache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Cache struct {
	Dir        string
	TTLSeconds int
	Runner     func(repoDir string) (string, error)
	Now        func() time.Time
}

type cachedEntry struct {
	Stamp      time.Time `json:"stamp"`
	HEADmtime  time.Time `json:"head_mtime"`
	IndexMtime time.Time `json:"index_mtime"`
	Git        Git       `json:"git"`
}

// DefaultRunner runs `git --no-optional-locks status --porcelain=v2 --branch
// --ahead-behind` in repoDir. --no-optional-locks is a top-level git flag,
// not a status flag, so it must come before the subcommand.
func DefaultRunner(repoDir string) (string, error) {
	cmd := exec.Command("git",
		"-C", repoDir,
		"-c", "core.fsmonitor=false",
		"--no-optional-locks",
		"status",
		"--porcelain=v2",
		"--branch",
		"--ahead-behind",
	)
	out, err := cmd.Output()
	return string(out), err
}

// Query returns parsed git state for the repo containing cwd, or nil if cwd
// is not inside a git repository. Reads from cache if TTL hasn't elapsed and
// neither .git/HEAD nor .git/index has changed mtime since the cached entry.
func (c *Cache) Query(cwd string) (*Git, error) {
	gitDir, ok := resolveGitDir(cwd)
	if !ok {
		return nil, nil
	}
	headInfo, _ := os.Stat(filepath.Join(gitDir, "HEAD"))
	indexInfo, _ := os.Stat(filepath.Join(gitDir, "index"))
	cachePath := c.cachePath(gitDir)
	if entry, ok := c.readCache(cachePath); ok {
		fresh := c.Now().Sub(entry.Stamp) < time.Duration(c.TTLSeconds)*time.Second
		if fresh && sameTime(headInfo, entry.HEADmtime) && sameTime(indexInfo, entry.IndexMtime) {
			g := entry.Git
			return &g, nil
		}
	}
	repoDir := repoRoot(gitDir, cwd)
	out, err := c.Runner(repoDir)
	if err != nil {
		return nil, err
	}
	parsed := ParsePorcelainV2(out)
	entry := cachedEntry{
		Stamp:      c.Now(),
		HEADmtime:  modTime(headInfo),
		IndexMtime: modTime(indexInfo),
		Git:        parsed,
	}
	c.writeCache(cachePath, entry)
	return &parsed, nil
}

func resolveGitDir(cwd string) (string, bool) {
	dir := cwd
	for {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Lstat(gitPath)
		if err == nil {
			if info.IsDir() {
				return gitPath, true
			}
			data, err := os.ReadFile(gitPath)
			if err != nil {
				return "", false
			}
			line := strings.TrimSpace(string(data))
			line = strings.TrimPrefix(line, "gitdir:")
			resolved := strings.TrimSpace(line)
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(dir, resolved)
			}
			return resolved, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func repoRoot(gitDir, cwd string) string {
	if strings.Contains(gitDir, ".git"+string(filepath.Separator)+"worktrees") {
		return cwd
	}
	return filepath.Dir(gitDir)
}

func (c *Cache) cachePath(gitDir string) string {
	h := sha256.Sum256([]byte(gitDir))
	return filepath.Join(c.Dir, hex.EncodeToString(h[:16])+".json")
}

func (c *Cache) readCache(path string) (cachedEntry, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return cachedEntry{}, false
	}
	var e cachedEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return cachedEntry{}, false
	}
	return e, true
}

func (c *Cache) writeCache(path string, e cachedEntry) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	tmp := path + ".tmp"
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func modTime(info os.FileInfo) time.Time {
	if info == nil {
		return time.Time{}
	}
	return info.ModTime()
}

func sameTime(info os.FileInfo, t time.Time) bool {
	return modTime(info).Equal(t)
}
