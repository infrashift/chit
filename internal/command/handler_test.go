package command

import (
	"context"
	"testing"
)

func TestHandlerFuncExecute(t *testing.T) {
	called := false
	hf := HandlerFunc(func(ctx context.Context, actorID, channelID, args string) (*CommandResult, error) {
		called = true
		if actorID != "user-1" {
			t.Errorf("unexpected actorID: %s", actorID)
		}
		if channelID != "ch-1" {
			t.Errorf("unexpected channelID: %s", channelID)
		}
		if args != "test args" {
			t.Errorf("unexpected args: %s", args)
		}
		return &CommandResult{ResponseText: "ok", Triggered: true}, nil
	})

	result, err := hf.Execute(context.Background(), "user-1", "ch-1", "test args")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called")
	}
	if result.ResponseText != "ok" {
		t.Errorf("unexpected response: %s", result.ResponseText)
	}
	if !result.Triggered {
		t.Error("expected Triggered=true")
	}
}
