package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ClaudeResult is the subset of `claude -p --output-format json` output the
// bridge consumes.
type ClaudeResult struct {
	Result       string      `json:"result"`
	SessionID    string      `json:"session_id"`
	IsError      bool        `json:"is_error"`
	NumTurns     int         `json:"num_turns"`
	TotalCostUSD float64     `json:"total_cost_usd"`
	Usage        ClaudeUsage `json:"usage"`
}

// ClaudeUsage carries token accounting for the footer.
type ClaudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
}

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
// The prompt is passed as an argv element — never through a shell.
func (r *ClaudeRunner) Run(ctx context.Context, prompt, sessionID string) (*ClaudeResult, error) {
	runCtx, cancel := context.WithTimeout(ctx, r.cfg.RunTimeout)
	defer cancel()

	args := []string{
		"-p", prompt,
		"--output-format", "json",
		"--permission-mode", r.cfg.PermissionMode,
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

	cmd := exec.CommandContext(runCtx, r.cfg.ClaudeBin, args...)
	cmd.Dir = r.cfg.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("claude run timed out after %s", r.cfg.RunTimeout)
		}
		return nil, fmt.Errorf("claude run failed: %w: %s", err, tail(stderr.String(), 500))
	}

	var result ClaudeResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("parse claude output: %w: %s", err, tail(stdout.String(), 500))
	}

	return &result, nil
}

// Footer renders the usage summary appended to every agent reply.
func (r *ClaudeRunner) Footer(res *ClaudeResult) string {
	contextTokens := res.Usage.CacheReadInputTokens + res.Usage.InputTokens
	pct := 0
	if r.cfg.ContextWindow > 0 {
		pct = int(contextTokens * 100 / int64(r.cfg.ContextWindow))
	}
	return fmt.Sprintf("\n\n---\n`⚙ tokens in %s · out %s · cache %s · $%.2f · context ~%d%%`",
		humanTokens(res.Usage.InputTokens),
		humanTokens(res.Usage.OutputTokens),
		humanTokens(res.Usage.CacheReadInputTokens),
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

// tail returns at most n trailing characters of s, trimmed.
func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}
