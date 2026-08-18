package transcript

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseTailExtractsTokensFromAssistantMessages(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("msg-1", now.Add(-30*time.Second), usage{input: 1500, cacheCreate: 200, cacheRead: 8000, output: 100}, nil),
		assistantLine("msg-2", now.Add(-10*time.Second), usage{input: 2000, output: 200}, nil),
	}
	path := writeJSONL(t, lines)
	entries, err := ParseTail(path, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(entries.Requests))
	}
	if entries.Requests[0].InputTokens != 1500 || entries.Requests[0].CacheRead != 8000 {
		t.Errorf("first request: %+v", entries.Requests[0])
	}
}

func TestParseTailDedupesAssistantByMessageID(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	// Streaming can write the same message id multiple times with growing
	// token counts — the latest copy wins.
	lines := []string{
		assistantLine("msg-1", now.Add(-30*time.Second), usage{input: 1000, output: 50}, nil),
		assistantLine("msg-1", now.Add(-30*time.Second), usage{input: 1500, output: 100}, nil),
		assistantLine("msg-2", now.Add(-10*time.Second), usage{input: 2000, output: 200}, nil),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(entries.Requests))
	}
	if entries.Requests[0].InputTokens != 1500 {
		t.Errorf("dedup kept wrong copy: got %d input tokens", entries.Requests[0].InputTokens)
	}
}

func TestParseTailExtractsToolUsesFromContentBlocks(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("msg-1", now.Add(-20*time.Second), usage{input: 100}, []block{
			{Type: "tool_use", ID: "toolu_1", Name: "Read", Input: `{"file_path":"/foo/bar/main.go"}`},
		}),
		userResultLine(now.Add(-15*time.Second), "toolu_1", false),
		assistantLine("msg-2", now.Add(-10*time.Second), usage{input: 200}, []block{
			{Type: "tool_use", ID: "toolu_2", Name: "Bash", Input: `{"command":"go test ./...","description":"run tests"}`},
		}),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	// Read completed (has a tool_result) → ToolCounts; Bash is still running → Tools.
	if len(entries.Tools) != 1 {
		t.Fatalf("running tools = %d, want 1 (%+v)", len(entries.Tools), entries.Tools)
	}
	if entries.Tools[0].Name != "Bash" || !strings.Contains(entries.Tools[0].Target, "go test") {
		t.Errorf("running tool = %+v", entries.Tools[0])
	}
	if len(entries.ToolCounts) != 1 || entries.ToolCounts[0].Name != "Read" || entries.ToolCounts[0].Count != 1 {
		t.Errorf("tool counts = %+v, want Read ×1", entries.ToolCounts)
	}
}

func TestParseTailToolCountsAccumulate(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{}
	// Three completed Reads + one completed Bash, across separate turns.
	for i, n := range []string{"r1", "r2", "r3", "b1"} {
		name := "Read"
		if n == "b1" {
			name = "Bash"
		}
		ts := now.Add(time.Duration(-40+i*5) * time.Second)
		lines = append(lines,
			assistantLine("m"+n, ts, usage{input: 10}, []block{
				{Type: "tool_use", ID: "t" + n, Name: name, Input: `{}`},
			}),
			userResultLine(ts.Add(time.Second), "t"+n, false),
		)
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range entries.ToolCounts {
		got[c.Name] = c.Count
	}
	if got["Read"] != 3 || got["Bash"] != 1 {
		t.Errorf("counts = %+v, want Read 3 / Bash 1", got)
	}
}

func TestParseTailExtractsTodosFromTodoWrite(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	input := `{"todos":[{"subject":"refactor parser","status":"completed"},{"subject":"add tests","status":"in_progress"},{"subject":"docs","status":"pending"}]}`
	lines := []string{
		assistantLine("msg-1", now, usage{input: 100}, []block{
			{Type: "tool_use", ID: "toolu_t", Name: "TodoWrite", Input: input},
		}),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Todos) != 1 {
		t.Fatalf("todos snapshots = %d, want 1", len(entries.Todos))
	}
	if got := entries.Todos[0].Todos; len(got) != 3 || got[1].Status != "in_progress" {
		t.Errorf("todos = %+v", got)
	}
}

func TestParseTailTracksTasksFromCreateResults(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		// Create two tasks; their real ids come back in the results.
		assistantLine("a1", now.Add(-50*time.Second), usage{input: 50}, []block{
			{Type: "tool_use", ID: "tc1", Name: "TaskCreate", Input: `{"subject":"refactor parser"}`},
		}),
		userResultTextLine(now.Add(-49*time.Second), "tc1", "Task #22 created successfully: refactor parser"),
		assistantLine("a2", now.Add(-48*time.Second), usage{input: 50}, []block{
			{Type: "tool_use", ID: "tc2", Name: "TaskCreate", Input: `{"subject":"write tests"}`},
		}),
		userResultTextLine(now.Add(-47*time.Second), "tc2", "Task #23 created successfully: write tests"),
		// Now flip the second task to in_progress via its REAL id (23).
		assistantLine("a3", now.Add(-10*time.Second), usage{input: 50}, []block{
			{Type: "tool_use", ID: "tu1", Name: "TaskUpdate", Input: `{"taskId":"23","status":"in_progress"}`},
		}),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Todos) != 1 {
		t.Fatalf("expected 1 normalized todo snapshot, got %d", len(entries.Todos))
	}
	todos := entries.Todos[0].Todos
	if len(todos) != 2 {
		t.Fatalf("expected 2 tracked tasks, got %d (%+v)", len(todos), todos)
	}
	// Task #23 ("write tests") should be in_progress; #22 still pending.
	if todos[0].Subject != "refactor parser" || todos[0].Status != "pending" {
		t.Errorf("task[0] = %+v", todos[0])
	}
	if todos[1].Subject != "write tests" || todos[1].Status != "in_progress" {
		t.Errorf("task[1] = %+v, want write tests/in_progress", todos[1])
	}
}

func TestParseTailResetsToolCountsAtCompaction(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	mk := func(id string, ts time.Time, promptInput int, tool string) []string {
		return []string{
			assistantLine(id, ts, usage{input: promptInput}, []block{
				{Type: "tool_use", ID: "t" + id, Name: tool, Input: `{}`},
			}),
			userResultLine(ts.Add(time.Second), "t"+id, false),
		}
	}
	var lines []string
	// Epoch 1: prompt grows 50k → 90k, two Reads + one Bash completed.
	lines = append(lines, mk("m1", now.Add(-60*time.Second), 50_000, "Read")...)
	lines = append(lines, mk("m2", now.Add(-55*time.Second), 70_000, "Read")...)
	lines = append(lines, mk("m3", now.Add(-50*time.Second), 90_000, "Bash")...)
	// Compaction: prompt drops to 12k (well under 0.6×90k). One Edit after.
	lines = append(lines, mk("m4", now.Add(-10*time.Second), 12_000, "Edit")...)

	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, c := range entries.ToolCounts {
		got[c.Name] = c.Count
	}
	// Only the post-compaction Edit should remain; pre-compaction Reads/Bash reset.
	if got["Edit"] != 1 {
		t.Errorf("Edit count = %d, want 1", got["Edit"])
	}
	if got["Read"] != 0 || got["Bash"] != 0 {
		t.Errorf("pre-compaction counts should reset, got %+v", got)
	}
}

func TestParseTailResetsAtCompactionMarker(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	var lines []string
	// Epoch 1: a completed Read plus an in-progress todo list.
	lines = append(lines,
		assistantLine("m1", now.Add(-60*time.Second), usage{input: 50_000}, []block{
			{Type: "tool_use", ID: "tt1", Name: "Read", Input: `{}`},
			{Type: "tool_use", ID: "tw1", Name: "TodoWrite",
				Input: `{"todos":[{"content":"old task","status":"in_progress"}]}`},
		}),
		userResultLine(now.Add(-59*time.Second), "tt1", false),
	)
	// Explicit /compact marker: a user line flagged isCompactSummary. It must
	// reset tools AND drop todos immediately — without waiting for a new
	// assistant message (so a bare 1s refresh reflects the compaction).
	lines = append(lines, compactSummaryLine(now.Add(-30*time.Second)))

	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.ToolCounts) != 0 {
		t.Errorf("tool counts should reset at marker, got %+v", entries.ToolCounts)
	}
	// Todos have their own lifecycle and must survive a compaction.
	if len(entries.Todos) == 0 {
		t.Errorf("todos should persist across compaction, got none")
	}
}

func TestParseTailEmptyTodoWriteClearsTodos(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("m1", now, usage{input: 100}, []block{
			{Type: "tool_use", ID: "tw1", Name: "TodoWrite",
				Input: `{"todos":[{"content":"task","status":"in_progress"}]}`},
		}),
		// Claude clears the list with an empty TodoWrite — the line must drop.
		assistantLine("m2", now.Add(time.Second), usage{input: 100}, []block{
			{Type: "tool_use", ID: "tw2", Name: "TodoWrite", Input: `{"todos":[]}`},
		}),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Todos) != 0 {
		t.Errorf("empty TodoWrite should clear todos, got %+v", entries.Todos)
	}
}

func TestParseTailKeepsRecentlyCompletedTool(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("m1", now, usage{input: 100}, []block{
			{Type: "tool_use", ID: "t1", Name: "Bash", Input: `{"command":"go test ./..."}`},
		}),
		userResultLine(now.Add(2*time.Second), "t1", false),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Tools) != 0 {
		t.Errorf("a completed tool should leave the running list, got %+v", entries.Tools)
	}
	if len(entries.RecentTools) != 1 {
		t.Fatalf("want 1 recently-completed tool, got %d", len(entries.RecentTools))
	}
	rt := entries.RecentTools[0]
	if rt.Name != "Bash" || rt.EndedAt.IsZero() {
		t.Errorf("recent tool should be a completed Bash with EndedAt set, got %+v", rt)
	}
}

func TestParseTailCompletesBackgroundAgentOnNotification(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("m1", now, usage{input: 100}, []block{
			{Type: "tool_use", ID: "toolu_x", Name: "Agent",
				Input: `{"subagent_type":"Explore","description":"inventory","run_in_background":true}`},
		}),
		// The immediate launch result must NOT complete a background agent.
		userResultTextLine(now.Add(time.Second), "toolu_x", "Async agent launched successfully. agentId: abc"),
		// The async completion arrives later as a queue-operation notification.
		queueNotificationLine(now.Add(30*time.Second), "toolu_x", "completed"),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Agents) != 1 {
		t.Fatalf("want 1 agent, got %d: %+v", len(entries.Agents), entries.Agents)
	}
	if entries.Agents[0].EndedAt.IsZero() {
		t.Errorf("background agent should be completed by its task-notification")
	}
}

func TestParseTailBackgroundAgentStaysRunningWithoutNotification(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	lines := []string{
		assistantLine("m1", now, usage{input: 100}, []block{
			{Type: "tool_use", ID: "toolu_y", Name: "Agent",
				Input: `{"subagent_type":"Explore","description":"x","run_in_background":true}`},
		}),
		userResultTextLine(now.Add(time.Second), "toolu_y", "Async agent launched successfully."),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Agents) != 1 {
		t.Fatalf("want 1 agent, got %d", len(entries.Agents))
	}
	if !entries.Agents[0].EndedAt.IsZero() {
		t.Errorf("background agent must stay running until its completion notification")
	}
}

func TestParseTailHandlesPartialFirstLine(t *testing.T) {
	body := strings.Repeat("x", 65*1024) + "\n" +
		assistantLine("msg-1", time.Now(), usage{input: 100}, nil) + "\n"
	path := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := ParseTail(path, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Requests) != 1 {
		t.Errorf("requests = %d, want 1", len(entries.Requests))
	}
}

func TestParseTailMissingFile(t *testing.T) {
	entries, err := ParseTail(filepath.Join(t.TempDir(), "nope.jsonl"), 64*1024)
	if err != nil {
		t.Errorf("missing file should not error, got %v", err)
	}
	if entries != nil && (len(entries.Requests) != 0 || len(entries.Tools) != 0) {
		t.Errorf("expected empty entries, got %+v", entries)
	}
}

// ---- helpers ----

type usage struct {
	input       int
	cacheCreate int
	cacheRead   int
	output      int
}

type block struct {
	Type      string
	ID        string
	Name      string
	Input     string // raw JSON
	ToolUseID string
}

func writeJSONL(t *testing.T, lines []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assistantLine(msgID string, ts time.Time, u usage, content []block) string {
	contentJSON := "[]"
	if len(content) > 0 {
		parts := make([]string, 0, len(content))
		for _, b := range content {
			input := b.Input
			if input == "" {
				input = "{}"
			}
			parts = append(parts, fmt.Sprintf(
				`{"type":%q,"id":%q,"name":%q,"input":%s}`,
				b.Type, b.ID, b.Name, input))
		}
		contentJSON = "[" + strings.Join(parts, ",") + "]"
	}
	return fmt.Sprintf(
		`{"type":"assistant","timestamp":%q,"message":{"id":%q,"role":"assistant","content":%s,"usage":{"input_tokens":%d,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":%d}}}`,
		ts.UTC().Format(time.RFC3339Nano), msgID, contentJSON, u.input, u.cacheCreate, u.cacheRead, u.output)
}

func userResultLine(ts time.Time, toolUseID string, isError bool) string {
	return fmt.Sprintf(
		`{"type":"user","timestamp":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"is_error":%t,"content":"ok"}]}}`,
		ts.UTC().Format(time.RFC3339Nano), toolUseID, isError)
}

func queueNotificationLine(ts time.Time, toolUseID, status string) string {
	content := fmt.Sprintf(
		"<task-notification>\n<tool-use-id>%s</tool-use-id>\n<status>%s</status>\n<summary>Agent done</summary>\n</task-notification>",
		toolUseID, status)
	cb, _ := json.Marshal(content)
	return fmt.Sprintf(
		`{"type":"queue-operation","operation":"enqueue","timestamp":%q,"content":%s}`,
		ts.UTC().Format(time.RFC3339Nano), string(cb))
}

func compactSummaryLine(ts time.Time) string {
	return fmt.Sprintf(
		`{"type":"user","isCompactSummary":true,"timestamp":%q,"message":{"role":"user","content":"This session is being continued from a previous conversation that ran out of context."}}`,
		ts.UTC().Format(time.RFC3339Nano))
}

func userResultTextLine(ts time.Time, toolUseID, text string) string {
	tb, _ := json.Marshal(text)
	return fmt.Sprintf(
		`{"type":"user","timestamp":%q,"message":{"role":"user","content":[{"type":"tool_result","tool_use_id":%q,"content":%s}]}}`,
		ts.UTC().Format(time.RFC3339Nano), toolUseID, string(tb))
}

func TestFlattenForDisplayFoldsMultilineTargets(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"single line untouched", "go test ./...", "go test ./..."},
		{"newline becomes arrow", "cd /tmp\nls -la", "cd /tmp↵ls -la"},
		{"crlf collapses to one arrow", "a\r\nb", "a↵b"},
		{"lone carriage return", "a\rb", "a↵b"},
		{"blank lines collapse", "a\n\n\nb", "a↵b"},
		{"continuation indent trimmed", "if x; then\n    echo hi\nfi", "if x; then↵echo hi↵fi"},
		{"trailing newline keeps arrow", "make\n", "make↵"},
		{"leading newline dropped", "\nmake", "make"},
		{"interior tab becomes space", "go\ttest", "go test"},
		{"control bytes dropped", "a\x00\x07b", "ab"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flattenForDisplay(tc.in)
			if got != tc.want {
				t.Errorf("flattenForDisplay(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.ContainsAny(got, "\n\r\t") {
				t.Errorf("flattenForDisplay(%q) left raw whitespace control: %q", tc.in, got)
			}
		})
	}
}

func TestParseTailFlattensMultilineBashCommand(t *testing.T) {
	now := time.Date(2026, 5, 26, 12, 0, 0, 0, time.UTC)
	cmd := "cd /home/joe/dotfiles\nnix fmt hosts\nexit 1"
	cb, _ := json.Marshal(map[string]string{"command": cmd})
	lines := []string{
		assistantLine("msg-1", now.Add(-10*time.Second), usage{input: 100}, []block{
			{Type: "tool_use", ID: "toolu_1", Name: "Bash", Input: string(cb)},
		}),
	}
	entries, err := ParseTail(writeJSONL(t, lines), 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries.Tools) != 1 {
		t.Fatalf("running tools = %d, want 1", len(entries.Tools))
	}
	target := entries.Tools[0].Target
	if strings.ContainsAny(target, "\n\r") {
		t.Errorf("Target still contains a raw newline: %q", target)
	}
	if !strings.Contains(target, returnArrow) {
		t.Errorf("Target missing return arrow: %q", target)
	}
	if want := "cd /home/joe/dotfiles↵nix fmt hosts↵exit 1"; target != want {
		t.Errorf("Target = %q, want %q", target, want)
	}
}
