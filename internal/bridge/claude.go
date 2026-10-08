package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// ClaudeResult is the subset of `claude -p --output-format json` output the
// bridge consumes.
type ClaudeResult struct {
	Result       string                `json:"result"`
	SessionID    string                `json:"session_id"`
	IsError      bool                  `json:"is_error"`
	Subtype      string                `json:"subtype"`
	NumTurns     int                   `json:"num_turns"`
	TotalCostUSD float64               `json:"total_cost_usd"`
	Usage        ClaudeUsage           `json:"usage"`
	ModelUsage   map[string]ModelUsage `json:"modelUsage"`
}

// ClaudeUsage carries token accounting. The top-level figures sum every API
// call the run made; Iterations holds one entry per call, so the last one is
// the size of the conversation as the run left it.
type ClaudeUsage struct {
	InputTokens              int64         `json:"input_tokens"`
	OutputTokens             int64         `json:"output_tokens"`
	CacheReadInputTokens     int64         `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64         `json:"cache_creation_input_tokens"`
	Iterations               []ClaudeUsage `json:"iterations,omitempty"`
}

// ModelUsage is the per-model entry of the result's modelUsage map.
type ModelUsage struct {
	ContextWindow int64 `json:"contextWindow"`
}

// ContextTokens estimates how much of the context window the conversation
// fills: everything the last API call read as input, whether fresh, from the
// cache, or written to it. Summing the run's totals instead would count the
// conversation once per call, and a run of several turns would report more
// than a full window.
func (u ClaudeUsage) ContextTokens() int64 {
	last := u
	if n := len(u.Iterations); n > 0 {
		last = u.Iterations[n-1]
	}
	return last.InputTokens + last.CacheReadInputTokens + last.CacheCreationInputTokens
}

// ErrSessionNotFound is returned when --resume names a session the CLI has no
// transcript for: the bridge's WORKDIR or ~/.claude changed since the thread
// started, or the session was never written.
var ErrSessionNotFound = errors.New("claude session not found")

// RunError is a failed run. Summary is safe to show in the channel; Detail
// holds the stderr tail for the log, where a path or environment detail does
// no harm.
type RunError struct {
	Summary string
	Detail  string
}

func (e *RunError) Error() string { return e.Summary }

// killWaitDelay is how long a cancelled run's output pipes may stay open
// before Run gives up on them.
const killWaitDelay = 5 * time.Second

// ClaudeRunner executes headless Claude Code runs.
type ClaudeRunner struct {
	cfg *Config
}

// NewClaudeRunner creates a runner from bridge configuration.
func NewClaudeRunner(cfg *Config) *ClaudeRunner {
	return &ClaudeRunner{cfg: cfg}
}

// Run executes one headless turn. A non-empty sessionID resumes that session;
// otherwise Claude starts a new one (its ID is in the returned result).
//
// The prompt goes in on stdin, never in argv. `-p` is a boolean flag and the
// prompt is the CLI's positional argument, so a post reading "--version" or
// "--continue" passed as an argument would be parsed as that option.
//
// A run the CLI reports as failed (is_error) returns its result alongside a
// *RunError carrying the CLI's own message, so the caller can still keep the
// session it names.
func (r *ClaudeRunner) Run(ctx context.Context, prompt, sessionID string) (*ClaudeResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, r.cfg.RunTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, r.cfg.ClaudeBin, r.args(sessionID)...)
	cmd.Dir = r.cfg.WorkDir
	cmd.Stdin = strings.NewReader(prompt)
	// Cancelling kills claude but not what it started (an MCP server, a
	// Bash tool's command); one of those holding stdout open would keep Run
	// waiting past the timeout. WaitDelay bounds that wait.
	cmd.WaitDelay = killWaitDelay

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return nil, &RunError{Summary: fmt.Sprintf("claude run timed out after %s", r.cfg.RunTimeout)}
	}

	// The CLI exits non-zero on an API error but still prints its result,
	// whose message is the useful one; stderr then holds only a log tag.
	var result ClaudeResult
	parseErr := json.Unmarshal(stdout.Bytes(), &result)
	if parseErr == nil && result.IsError {
		msg := strings.TrimSpace(result.Result)
		if msg == "" {
			msg = "claude reported an error (" + result.Subtype + ")"
		}
		return &result, &RunError{Summary: firstLine(msg, 300), Detail: tail(stderr.String(), 2000)}
	}
	if runErr != nil {
		detail := tail(stderr.String(), 2000)
		if sessionID != "" && strings.Contains(stderr.String(), "No conversation found with session ID") {
			return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
		}
		summary := "claude exited with " + runErr.Error()
		if line := firstLine(stderr.String(), 200); line != "" {
			summary += ": " + line
		}
		return nil, &RunError{Summary: summary, Detail: detail}
	}
	if parseErr != nil {
		return nil, &RunError{
			Summary: "could not read claude's output",
			Detail:  fmt.Sprintf("%v: %s", parseErr, tail(stdout.String(), 2000)),
		}
	}
	return &result, nil
}

// contextWindow returns the window to measure the footer's fill against: the
// largest one the run's models report, else CHIT_CLAUDE_CONTEXT_WINDOW.
func (r *ClaudeRunner) contextWindow(res *ClaudeResult) int64 {
	var window int64
	for _, m := range res.ModelUsage {
		window = max(window, m.ContextWindow)
	}
	if window == 0 {
		window = int64(r.cfg.ContextWindow)
	}
	return window
}

// args builds the CLI arguments for one run. The run is isolated from the
// host's own Claude Code setup: --strict-mcp-config means no MCP server loads
// unless CHIT_CLAUDE_MCP_CONFIG names it, and --setting-sources keeps the host
// user's settings (and the permissions, hooks and plugins they enable) out.
// Both matter twice over: what the agent may do, and the tool definitions that
// would otherwise be sent, unseen, with every turn.
func (r *ClaudeRunner) args(sessionID string) []string {
	args := []string{
		"-p",
		"--output-format", "json",
		"--permission-mode", r.cfg.PermissionMode,
		"--strict-mcp-config",
	}
	if r.cfg.MCPConfig != "" {
		args = append(args, "--mcp-config", r.cfg.MCPConfig)
	}
	if r.cfg.SettingSources != SettingSourcesAll {
		args = append(args, "--setting-sources", r.cfg.SettingSources)
	}
	if r.cfg.Model != "" {
		args = append(args, "--model", r.cfg.Model)
	}
	if r.cfg.AllowedTools != "" {
		args = append(args, "--allowedTools", r.cfg.AllowedTools)
	}
	if r.cfg.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", r.cfg.AppendSystemPrompt)
	}
	if sessionID != "" {
		args = append(args, "--resume", sessionID)
	}
	return args
}

// Footer renders the usage summary appended to every agent reply. Cache
// writes are shown apart from reads: a turn that finds the prompt cache cold
// rewrites the whole conversation into it, and that is where a long thread's
// cost goes.
func (r *ClaudeRunner) Footer(res *ClaudeResult) string {
	pct := int64(0)
	if window := r.contextWindow(res); window > 0 {
		pct = res.Usage.ContextTokens() * 100 / window
	}
	return fmt.Sprintf("\n\n---\n`⚙ in %s · out %s · cache read %s · write %s · $%.2f · context ~%d%%`",
		humanTokens(res.Usage.InputTokens),
		humanTokens(res.Usage.OutputTokens),
		humanTokens(res.Usage.CacheReadInputTokens),
		humanTokens(res.Usage.CacheCreationInputTokens),
		res.TotalCostUSD,
		pct,
	)
}

func humanTokens(n int64) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

// tail returns at most n trailing bytes of s, trimmed, never splitting a
// UTF-8 sequence.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	i := len(s) - n
	for i < len(s) && !utf8.RuneStart(s[i]) {
		i++
	}
	return "…" + s[i:]
}

// firstLine returns the first non-blank line of s, cut to at most n bytes.
func firstLine(s string, n int) string {
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return truncate(line, n)
		}
	}
	return ""
}

// truncate cuts s to at most n bytes on a rune boundary, marking the cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	i := n
	for i > 0 && !utf8.RuneStart(s[i]) {
		i--
	}
	return s[:i] + "…"
}
