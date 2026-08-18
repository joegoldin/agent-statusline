// Package gitcache runs git status --porcelain=v2 once per session,
// parses the output into a structured form, and caches the result on disk.
package gitcache

import (
	"strconv"
	"strings"
)

type Git struct {
	Branch    string
	SHA       string
	Upstream  string
	Ahead     int
	Behind    int
	Detached  bool
	Modified  int
	Staged    int
	Untracked int
	Conflicts int
	Dirty     bool
}

// ParsePorcelainV2 parses `git status --porcelain=v2 --branch --ahead-behind`.
// Reference: https://git-scm.com/docs/git-status#_porcelain_format_version_2
func ParsePorcelainV2(out string) Git {
	var g Git
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.oid "):
			g.SHA = strings.TrimPrefix(line, "# branch.oid ")
		case strings.HasPrefix(line, "# branch.head "):
			head := strings.TrimPrefix(line, "# branch.head ")
			if head == "(detached)" {
				g.Detached = true
			} else {
				g.Branch = head
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			g.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			parts := strings.Fields(strings.TrimPrefix(line, "# branch.ab "))
			if len(parts) == 2 {
				g.Ahead, _ = strconv.Atoi(strings.TrimPrefix(parts[0], "+"))
				g.Behind, _ = strconv.Atoi(strings.TrimPrefix(parts[1], "-"))
			}
		case strings.HasPrefix(line, "1 "):
			if len(line) < 4 {
				continue
			}
			xy := line[2:4]
			if xy[0] != '.' {
				g.Staged++
			}
			if xy[1] != '.' {
				g.Modified++
			}
		case strings.HasPrefix(line, "2 "):
			// Renamed/copied entries: skip from staged/modified counts (test
			// fixture and ccstatusline both treat these as out-of-scope for
			// the headline file counts).
		case strings.HasPrefix(line, "? "):
			g.Untracked++
		case strings.HasPrefix(line, "u "):
			// Conflicts have both sides changed; count as worktree modification
			// in addition to the conflict counter.
			g.Conflicts++
			if len(line) >= 4 && line[3] != '.' {
				g.Modified++
			}
		}
	}
	g.Dirty = g.Modified+g.Staged+g.Untracked+g.Conflicts > 0
	return g
}
