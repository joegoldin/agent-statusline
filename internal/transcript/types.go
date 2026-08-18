// Package transcript reads the trailing portion of Claude Code's JSONL
// transcript file. We only ever need recent entries: enough to compute a
// rolling burn rate plus the most recent tool/agent/todo activity.
package transcript

import "time"

// Entries is the result of parsing the transcript. Tool data is split:
// Tools holds only currently-running tools (no tool_result yet) for the
// running-tools row, while ToolCounts holds session-total completed counts
// per tool name so the recent-tools row increments across the whole session.
type Entries struct {
	Requests    []Request
	Tools       []Tool      // currently running (uncompleted)
	RecentTools []Tool      // recently completed (EndedAt set), for a brief linger
	ToolCounts  []ToolCount // completed, aggregated by name, session-total
	Agents      []Agent
	Todos       []TodoSnapshot
}

// ToolCount is a completed-tool aggregate: how many times a tool with this
// name finished over the session.
type ToolCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type Request struct {
	ID            string
	RequestID     string
	Timestamp     time.Time
	InputTokens   int
	CacheCreate   int
	CacheRead     int
	OutputTokens  int
	ParentAgentID string
}

type Tool struct {
	ID        string
	Name      string
	Target    string
	Timestamp time.Time // tool_use time (start)
	EndedAt   time.Time // tool_result time; zero while running
}

type Agent struct {
	ID          string
	Name        string // subagent_type (e.g. "Explore", "general-purpose")
	Model       string // optional model override; empty when default
	Description string
	StartedAt   time.Time
	EndedAt     time.Time
	Background  bool // run_in_background — tool_result on launch can't be used as completion
}

type TodoSnapshot struct {
	Timestamp time.Time
	Todos     []TodoItem
}

type TodoItem struct {
	Subject string
	Status  string
}
