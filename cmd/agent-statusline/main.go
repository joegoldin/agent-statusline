package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/joegoldin/agent-statusline/internal/compaction"
	"github.com/joegoldin/agent-statusline/internal/config"
	"github.com/joegoldin/agent-statusline/internal/gitcache"
	"github.com/joegoldin/agent-statusline/internal/input"
	"github.com/joegoldin/agent-statusline/internal/layout"
	"github.com/joegoldin/agent-statusline/internal/render"
	"github.com/joegoldin/agent-statusline/internal/toolclock"
	"github.com/joegoldin/agent-statusline/internal/transcript"
	"github.com/joegoldin/agent-statusline/internal/voice"
	"github.com/joegoldin/agent-statusline/internal/widgets"
)

// dropPriority — lowest first (dropped first on overflow).
// Conversation-state widgets drop before identity/account widgets, so a
// narrow terminal keeps the "who/where/budget" header intact.
var dropPriority = []string{
	"sessionName", "compaction", "pr", "voice", "cost",
	"burnRate", "duration", "tokens", "effort", "context",
	"usage7d", "usage5h", "git", "cwd", "model",
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			debugLog("panic in main: %v", r)
			os.Exit(0)
		}
	}()

	// `claude-statusline hook` runs as a Claude Code tool hook (PermissionRequest
	// / PostToolUse / PostToolUseFailure), recording real tool start/end times to
	// the per-session sidecar the statusline reads. Same binary so the sidecar
	// format and cache path live in one place.
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runToolHook()
		return
	}

	status, err := input.Decode(os.Stdin)
	if err != nil {
		debugLog("input.Decode: %v", err)
		os.Exit(0)
	}

	cfg, err := config.Load(config.ResolvePath())
	if err != nil {
		debugLog("config.Load: %v", err)
	}

	cacheRoot := userCacheDir()

	ctx := &widgets.Context{
		Status: status,
		Cfg:    cfg,
		Now:    resolveNow(),
	}

	ctx.GitProvider = memoize(func() *gitcache.Git {
		c := &gitcache.Cache{
			Dir:        filepath.Join(cacheRoot, "git"),
			TTLSeconds: cfg.GitCacheTTLSeconds,
			Runner:     gitcache.DefaultRunner,
			Now:        time.Now,
		}
		g, err := c.Query(status.CWD)
		if err != nil {
			debugLog("gitcache.Query: %v", err)
		}
		return g
	})
	ctx.TranscriptProvider = memoize(func() *transcript.Entries {
		if status.TranscriptPath == "" {
			return &transcript.Entries{}
		}
		window := time.Duration(cfg.TranscriptWindowSeconds) * time.Second
		if window <= 0 {
			window = 300 * time.Second
		}
		e, err := transcript.Load(
			status.TranscriptPath,
			filepath.Join(cacheRoot, "transcript"),
			ctx.Now,
			window,
		)
		if err != nil {
			debugLog("transcript.Load: %v", err)
			return &transcript.Entries{}
		}
		return e
	})
	ctx.VoiceProvider = memoize(func() *voice.Config {
		return voice.NewReader().Read(status.CWD)
	})
	ctx.CompactionProvider = memoize(func() int {
		if status.SessionID == "" || status.ContextWindow == nil || status.ContextWindow.UsedPercentage == nil {
			return 0
		}
		store := &compaction.Store{Dir: filepath.Join(cacheRoot, "compactions")}
		n, err := store.Track(status.SessionID, *status.ContextWindow.UsedPercentage, status.ContextWindow.ContextWindowSize)
		if err != nil {
			debugLog("compaction.Track: %v", err)
		}
		return n
	})
	ctx.ToolTimingProvider = memoize(func() map[string]toolclock.Entry {
		return toolclock.Load(cacheRoot, status.SessionID)
	})

	registry := buildRegistry()
	width := layout.DetectWidth()
	ctx.Width = width

	row1Widgets := resolveRow(cfg.Widgets.Row1, registry)
	row2Widgets := resolveRow(cfg.Widgets.Row2, registry)

	// Total line budget. Dashboard rows take priority; activity rows fill
	// whatever's left, so on a narrow terminal we wrap the dashboard onto
	// extra lines rather than truncate it, and drop activity rows first.
	const maxLines = 6

	// Build the dashboard. If row1+row2 fit on one line, join them
	// left-aligned with the normal segment separator. Otherwise WRAP each
	// row across as many lines as needed — never truncate the dashboard.
	naturalOpts := layout.Options{Width: 0, DropPriority: dropPriority, Hide: cfg.Widgets.Hide}
	row1Full := layout.ComposeRow(row1Widgets, nil, ctx, naturalOpts)
	row2Full := layout.ComposeRow(row2Widgets, nil, ctx, naturalOpts)
	row1W := render.VisibleWidth(row1Full)
	row2W := render.VisibleWidth(row2Full)
	sepW := render.VisibleWidth(layout.Separator)

	var dashboard []string
	if row1W > 0 && row2W > 0 && row1W+sepW+row2W <= width {
		dashboard = []string{row1Full + layout.Separator + row2Full}
	} else {
		wrapOpts := layout.Options{Width: width, Hide: cfg.Widgets.Hide}
		dashboard = append(dashboard, layout.WrapRow(row1Widgets, ctx, wrapOpts)...)
		dashboard = append(dashboard, layout.WrapRow(row2Widgets, ctx, wrapOpts)...)
	}
	if len(dashboard) > maxLines {
		dashboard = dashboard[:maxLines]
	}

	// Activity rows fill the remaining budget. Lower priority than the
	// dashboard, so they're truncated (not wrapped) and dropped first.
	activityBudget := maxLines - len(dashboard)
	if activityBudget > cfg.ActivityRows {
		activityBudget = cfg.ActivityRows
	}
	var activity []string
	if activityBudget > 0 {
		actWidgets := []widgets.Widget{widgets.Tools{}, widgets.ToolsRecent{}, widgets.Agents{}, widgets.Todos{}}
		for _, w := range actWidgets {
			if len(activity) >= activityBudget {
				break
			}
			text, vis := widgets.SafeRender(w, ctx)
			if !vis {
				continue
			}
			if width > 0 {
				text = render.Truncate(text, width)
			}
			activity = append(activity, text)
		}
	}

	lines := append(dashboard, activity...)
	out := strings.Builder{}
	for i, line := range lines {
		out.WriteString("\x1b[0m")
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
	}
	_, _ = os.Stdout.WriteString(out.String())
}

func resolveRow(names []string, reg widgets.Registry) []widgets.Widget {
	var out []widgets.Widget
	for _, n := range names {
		if n == layout.FlexName {
			out = append(out, layout.FlexMarker())
			continue
		}
		if w := reg.Lookup(n); w != nil {
			out = append(out, w)
		}
	}
	return out
}

func buildRegistry() widgets.Registry {
	all := []widgets.Widget{
		widgets.Model{}, widgets.CWD{}, widgets.Git{}, widgets.ContextBar{},
		widgets.Cost{}, widgets.Duration{}, widgets.Tokens{},
		widgets.Usage5h{}, widgets.Usage7d{}, widgets.BurnRate{},
		widgets.Effort{}, widgets.Voice{}, widgets.Compaction{}, widgets.PR{},
		widgets.SessionName{},
	}
	r := widgets.Registry{}
	for _, w := range all {
		r[w.Name()] = w
	}
	return r
}

// resolveNow returns time.Now(), or a fixed Unix-seconds time when
// CLAUDE_STATUSLINE_NOW is set (used by golden tests for deterministic output).
func resolveNow() time.Time {
	if s := os.Getenv("CLAUDE_STATUSLINE_NOW"); s != "" {
		var unix int64
		if _, err := fmt.Sscanf(s, "%d", &unix); err == nil && unix > 0 {
			return time.Unix(unix, 0).UTC()
		}
	}
	return time.Now()
}

func memoize[T any](f func() T) func() T {
	var once sync.Once
	var v T
	return func() T {
		once.Do(func() { v = f() })
		return v
	}
}

func userCacheDir() string {
	if d, err := os.UserCacheDir(); err == nil {
		return filepath.Join(d, "claude-statusline")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "claude-statusline")
}

// toolHookInput is the subset of a PermissionRequest / PostToolUse /
// PostToolUseFailure hook payload we need: which session, which tool, and
// which event. tool_use_id matches the transcript's tool_use block id.
type toolHookInput struct {
	SessionID     string `json:"session_id"`
	ToolUseID     string `json:"tool_use_id"`
	HookEventName string `json:"hook_event_name"`
}

// runToolHook reads a hook payload from stdin and records the tool's start
// (PermissionRequest) or end (PostToolUse / PostToolUseFailure) in the
// per-session sidecar. It always exits 0 and writes nothing to stdout: a
// statusline-timing hook must never block, fail, or interfere with the tool
// call it observes.
func runToolHook() {
	defer func() {
		_ = recover()
		os.Exit(0)
	}()
	var h toolHookInput
	if err := json.NewDecoder(os.Stdin).Decode(&h); err != nil {
		debugLog("hook decode: %v", err)
		return
	}
	if err := toolclock.Record(userCacheDir(), h.SessionID, h.HookEventName, h.ToolUseID, time.Now()); err != nil {
		debugLog("toolclock.Record: %v", err)
	}
}

func debugLog(format string, args ...any) {
	if os.Getenv("CLAUDE_STATUSLINE_DEBUG") == "" {
		return
	}
	path := filepath.Join(userCacheDir(), "debug.log")
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = fmt.Fprintf(f, time.Now().Format(time.RFC3339)+" "+format+"\n", args...)
}
