package command

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLoggerWritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")

	al, err := NewAuditLogger(path)
	if err != nil {
		t.Fatalf("NewAuditLogger: %v", err)
	}
	defer func() { _ = al.Close() }()

	al.LogCommandAttempt("evt-1", "user-abc", "user", "help", true)
	al.LogCommandAttempt("evt-2", "user-xyz", "agent", "kick", false)

	if err = al.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	lines := splitJSONLines(data)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	// Check first record.
	var rec map[string]any
	if err := json.Unmarshal(lines[0], &rec); err != nil {
		t.Fatalf("unmarshal line 0: %v", err)
	}
	if rec["event_id"] != "evt-1" {
		t.Errorf("expected event_id=evt-1, got %v", rec["event_id"])
	}
	if rec["actor_id"] != "user-abc" {
		t.Errorf("expected actor_id=user-abc, got %v", rec["actor_id"])
	}
	if rec["command_slug"] != "help" {
		t.Errorf("expected command_slug=help, got %v", rec["command_slug"])
	}
	if rec["authorized"] != true {
		t.Errorf("expected authorized=true, got %v", rec["authorized"])
	}

	// Check second record.
	var rec2 map[string]any
	if err := json.Unmarshal(lines[1], &rec2); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if rec2["authorized"] != false {
		t.Errorf("expected authorized=false, got %v", rec2["authorized"])
	}
}

func TestAuditLoggerStderr(t *testing.T) {
	al, err := NewAuditLogger("")
	if err != nil {
		t.Fatalf("NewAuditLogger stderr: %v", err)
	}
	defer func() { _ = al.Close() }()

	// Should not panic writing to stderr.
	al.LogCommandAttempt("evt-x", "u-1", "user", "topic", true)
}

func splitJSONLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			line := data[start:i]
			if len(line) > 0 {
				lines = append(lines, line)
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}
