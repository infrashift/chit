package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// writeScript writes an executable shell script named claude into dir.
func writeScript(t *testing.T, dir, body string) string {
	t.Helper()
	bin := filepath.Join(dir, "claude")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// A run of several API calls reports usage summed over all of them, which
// counts the conversation once per call. The context estimate must come from
// the last call alone, against the window the CLI reports for the model.
func TestClaudeRunner_FooterMultiTurnContext(t *testing.T) {
	runner := NewClaudeRunner(testConfig(t, "http://unused", "claude"))
	var res ClaudeResult
	if err := json.Unmarshal([]byte(`{
		"usage": {
			"input_tokens": 30, "output_tokens": 900,
			"cache_read_input_tokens": 250000, "cache_creation_input_tokens": 59990,
			"iterations": [
				{"input_tokens": 10, "cache_read_input_tokens": 80000, "cache_creation_input_tokens": 40000},
				{"input_tokens": 10, "cache_read_input_tokens": 80000, "cache_creation_input_tokens": 10000},
				{"input_tokens": 10, "cache_read_input_tokens": 90000, "cache_creation_input_tokens": 9990}
			]
		},
		"modelUsage": {
			"claude-haiku-4-5": {"contextWindow": 200000},
			"claude-opus-5-5": {"contextWindow": 1000000}
		}
	}`), &res); err != nil {
		t.Fatal(err)
	}
	// Last call: 10 + 90000 + 9990 = 100000 of the largest window, 1M. The
	// summed totals would have claimed 31%, or 155% of a 200k window.
	if footer := runner.Footer(&res); !strings.Contains(footer, "context ~10%") {
		t.Errorf("want context ~10%%: %s", footer)
	}
}

// On an API error the CLI exits non-zero but still prints a result whose
// message is the one to show; stderr then holds only a log tag.
func TestClaudeRunner_IsErrorResultUsesItsMessage(t *testing.T) {
	bin := writeScript(t, t.TempDir(), `cat > /dev/null
echo '[claude-code:unrecognized_model] {"model":"nope"}' >&2
cat <<'FAKEEOF'
{"type":"result","subtype":"success","is_error":true,"session_id":"`+fakeSessionID+`",
 "result":"There's an issue with the selected model (nope). It may not exist."}
FAKEEOF
exit 1
`)
	res, err := NewClaudeRunner(testConfig(t, "http://unused", bin)).Run(context.Background(), "hi", "")
	var runErr *RunError
	if !errors.As(err, &runErr) {
		t.Fatalf("want *RunError, got %v", err)
	}
	if !strings.Contains(runErr.Summary, "issue with the selected model") {
		t.Errorf("summary should be the CLI's message, got %q", runErr.Summary)
	}
	if strings.Contains(runErr.Summary, "unrecognized_model") {
		t.Errorf("stderr belongs in Detail, not the summary: %q", runErr.Summary)
	}
	if res == nil || res.SessionID != fakeSessionID {
		t.Errorf("the failed run's result (and session) should come back, got %+v", res)
	}
}

// A run that exits 0 but reports is_error (a turn limit, say) is a failure
// too, not an "_(empty response)_".
func TestClaudeRunner_IsErrorWithCleanExit(t *testing.T) {
	bin := writeScript(t, t.TempDir(), `cat > /dev/null
echo '{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"s-1","result":""}'
`)
	_, err := NewClaudeRunner(testConfig(t, "http://unused", bin)).Run(context.Background(), "hi", "")
	if err == nil || !strings.Contains(err.Error(), "error_max_turns") {
		t.Errorf("want an error naming the subtype, got %v", err)
	}
}

func TestClaudeRunner_SessionNotFound(t *testing.T) {
	bin := writeScript(t, t.TempDir(), `cat > /dev/null
echo "No conversation found with session ID: $*" >&2
exit 1
`)
	_, err := NewClaudeRunner(testConfig(t, "http://unused", bin)).Run(context.Background(), "hi", fakeSessionID)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("want ErrSessionNotFound, got %v", err)
	}
}

func TestClaudeRunner_Timeout(t *testing.T) {
	// The script's own child, not the script, holds stdout: cancelling kills
	// only the script, so Run returns through WaitDelay rather than waiting
	// out the child.
	bin := writeScript(t, t.TempDir(), "sleep 30\n")
	cfg := testConfig(t, "http://unused", bin)
	cfg.RunTimeout = 100 * time.Millisecond

	start := time.Now()
	_, err := NewClaudeRunner(cfg).Run(context.Background(), "hi", "")
	if err == nil || !strings.Contains(err.Error(), "timed out after 100ms") {
		t.Errorf("want a timeout error, got %v", err)
	}
	if took := time.Since(start); took > killWaitDelay+5*time.Second {
		t.Errorf("timeout took %s to return", took)
	}
}

func TestClaudeRunner_UnreadableOutput(t *testing.T) {
	bin := writeScript(t, t.TempDir(), "cat > /dev/null\necho 'not json'\n")
	_, err := NewClaudeRunner(testConfig(t, "http://unused", bin)).Run(context.Background(), "hi", "")
	var runErr *RunError
	if !errors.As(err, &runErr) || !strings.Contains(runErr.Detail, "not json") {
		t.Errorf("want a RunError with the output in Detail, got %v", err)
	}
}

func TestSplitContent(t *testing.T) {
	long := strings.Repeat("line of text\n", 40) + strings.Repeat("é", 300)
	for _, limit := range []int{64, 100, 257} {
		parts := splitContent(long, limit)
		if strings.Join(parts, "") != long {
			t.Fatalf("limit %d: parts do not rejoin to the original", limit)
		}
		for i, p := range parts {
			if len(p) > limit || p == "" || !utf8.ValidString(p) {
				t.Errorf("limit %d part %d: len %d, valid UTF-8 %v", limit, i, len(p), utf8.ValidString(p))
			}
		}
	}
	if got := splitContent("short", 64); len(got) != 1 || got[0] != "short" {
		t.Errorf("short content must stay whole, got %q", got)
	}
}

func TestTailAndFirstLineKeepRunesWhole(t *testing.T) {
	s := strings.Repeat("é", 10) // 20 bytes
	if got := tail(s, 5); !utf8.ValidString(got) {
		t.Errorf("tail split a rune: %q", got)
	}
	if got := firstLine("\n\n  "+s+"\nsecond", 5); !utf8.ValidString(got) || strings.Contains(got, "second") {
		t.Errorf("firstLine = %q", got)
	}
}
