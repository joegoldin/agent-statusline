package gitcache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestQueryCachesResult(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	repoDir := makeRepo(t, tmp)

	calls := 0
	runner := func(_ string) (string, error) {
		calls++
		return mockPorcelain(), nil
	}
	c := &Cache{
		Dir:        cacheDir,
		TTLSeconds: 60,
		Runner:     runner,
		Now:        func() time.Time { return time.Unix(1000, 0) },
	}

	g1, err := c.Query(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	g2, err := c.Query(repoDir)
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("expected git to run once, ran %d", calls)
	}
	if g1.Branch != "main" || g2.Branch != "main" {
		t.Errorf("branches: %q %q", g1.Branch, g2.Branch)
	}
}

func TestQueryRefreshesAfterTTL(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	repoDir := makeRepo(t, tmp)
	now := time.Unix(1000, 0)
	calls := 0
	c := &Cache{
		Dir:        cacheDir,
		TTLSeconds: 5,
		Runner: func(string) (string, error) {
			calls++
			return mockPorcelain(), nil
		},
		Now: func() time.Time { return now },
	}
	_, _ = c.Query(repoDir)
	now = now.Add(10 * time.Second)
	_, _ = c.Query(repoDir)
	if calls != 2 {
		t.Errorf("expected 2 runs, got %d", calls)
	}
}

func TestQueryRefreshesAfterHEADmtime(t *testing.T) {
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	repoDir := makeRepo(t, tmp)
	headPath := filepath.Join(repoDir, ".git", "HEAD")
	calls := 0
	c := &Cache{
		Dir:        cacheDir,
		TTLSeconds: 3600,
		Runner: func(string) (string, error) {
			calls++
			return mockPorcelain(), nil
		},
		Now: time.Now,
	}
	_, _ = c.Query(repoDir)
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(headPath, future, future); err != nil {
		t.Fatal(err)
	}
	_, _ = c.Query(repoDir)
	if calls != 2 {
		t.Errorf("expected 2 runs after HEAD touch, got %d", calls)
	}
}

func TestQueryNotARepoReturnsNil(t *testing.T) {
	tmp := t.TempDir()
	requireNoAncestorGitDir(t, tmp)
	c := &Cache{
		Dir:        filepath.Join(tmp, "cache"),
		TTLSeconds: 60,
		Runner: func(string) (string, error) {
			return "", nil
		},
		Now: time.Now,
	}
	g, err := c.Query(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if g != nil {
		t.Errorf("expected nil git for non-repo, got %+v", g)
	}
}

func makeRepo(t *testing.T, parent string) string {
	t.Helper()
	repo := filepath.Join(parent, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"HEAD", "index"} {
		if err := os.WriteFile(filepath.Join(repo, ".git", f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

func mockPorcelain() string {
	return strings.Join([]string{
		"# branch.oid abcdef1234567890",
		"# branch.head main",
	}, "\n") + "\n"
}

// requireNoAncestorGitDir skips when some ancestor of dir is a git repository.
// resolveGitDir walks upward, so a stray .git above the temp root (a /tmp/.git
// is enough) would make "not a repo" unrepresentable on that machine. The nix
// check sandbox always has a clean tmp, so CI still exercises this path.
func requireNoAncestorGitDir(t *testing.T, dir string) {
	t.Helper()
	for d := dir; ; {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			t.Skipf("ancestor %s is a git repository; cannot represent a non-repo path here", d)
		}
		parent := filepath.Dir(d)
		if parent == d {
			return
		}
		d = parent
	}
}
