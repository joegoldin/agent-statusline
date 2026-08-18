package gitcache

import "testing"

func TestParsePorcelainV2(t *testing.T) {
	out := `# branch.oid abcdef1234567890
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -1
1 .M N... 100644 100644 100644 aaa bbb file1.go
1 M. N... 100644 100644 100644 ccc ddd file2.go
1 A. N... 000000 100644 100644 0000 eee file3.go
2 R. N... 100644 100644 100644 fff fff R100 newname.go	oldname.go
? untracked.go
? another.go
u UU N... 100644 100644 100644 100644 xxx yyy zzz zzz conflict.go
`
	g := ParsePorcelainV2(out)
	if g.Branch != "main" {
		t.Errorf("branch = %q", g.Branch)
	}
	if g.SHA != "abcdef1234567890" {
		t.Errorf("sha = %q", g.SHA)
	}
	if g.Upstream != "origin/main" {
		t.Errorf("upstream = %q", g.Upstream)
	}
	if g.Ahead != 2 || g.Behind != 1 {
		t.Errorf("ahead/behind = %d/%d", g.Ahead, g.Behind)
	}
	if g.Modified != 2 {
		t.Errorf("modified = %d", g.Modified)
	}
	if g.Staged != 2 {
		t.Errorf("staged = %d", g.Staged)
	}
	if g.Untracked != 2 {
		t.Errorf("untracked = %d", g.Untracked)
	}
	if g.Conflicts != 1 {
		t.Errorf("conflicts = %d", g.Conflicts)
	}
	if !g.Dirty {
		t.Errorf("dirty should be true")
	}
}

func TestParsePorcelainV2DetachedHead(t *testing.T) {
	out := `# branch.oid abcdef1234567890
# branch.head (detached)
`
	g := ParsePorcelainV2(out)
	if g.Branch != "" {
		t.Errorf("branch should be empty for detached, got %q", g.Branch)
	}
	if !g.Detached {
		t.Errorf("Detached should be true")
	}
	if g.SHA != "abcdef1234567890" {
		t.Errorf("sha = %q", g.SHA)
	}
}

func TestParsePorcelainV2Clean(t *testing.T) {
	out := `# branch.oid abcdef1234567890
# branch.head main
`
	g := ParsePorcelainV2(out)
	if g.Dirty {
		t.Errorf("Dirty should be false for clean tree")
	}
	if g.Modified+g.Staged+g.Untracked+g.Conflicts != 0 {
		t.Errorf("counts should all be zero")
	}
}
