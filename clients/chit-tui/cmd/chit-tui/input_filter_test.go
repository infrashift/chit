package main

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func runes(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

// Bubble Tea splits a burst of typed text at spaces, and text inputs match
// keys by name, so a chunk that is exactly "right" moved the cursor instead
// of being typed: "turn right now" arrived as "turn  now". Found driving the
// client live. Several runes in one key are text, never a named key, so they
// are marked as pasted, which no shortcut matches.
func TestFilterInput_MultiRuneChunksAreText(t *testing.T) {
	for _, word := range []string{"right", "left", "up", "down", "home", "end", "enter"} {
		got, ok := filterInput(runes(word)).(tea.KeyMsg)
		if !ok {
			t.Fatalf("%q was dropped", word)
		}
		if !got.Paste {
			t.Errorf("%q not marked as text; it would match the %s key", word, word)
		}
		if got.String() == word {
			t.Errorf("%q still reads as the key name", word)
		}
	}
}

// Single keys are left alone, or j, k and d would stop being shortcuts.
func TestFilterInput_SingleKeysAreUntouched(t *testing.T) {
	for _, msg := range []tea.KeyMsg{runes("k"), {Type: tea.KeyRight}, {Type: tea.KeyEnter}} {
		if got := filterInput(msg); !reflect.DeepEqual(got, tea.Msg(msg)) {
			t.Errorf("%v changed to %v", msg, got)
		}
	}
}

func TestFilterInput_DropsTerminalResponses(t *testing.T) {
	for _, s := range []string{"]11;rgb:1a1a/1b1b/2626", "12;40R"} {
		if got := filterInput(runes(s)); got != nil {
			t.Errorf("terminal response %q got through as %v", s, got)
		}
	}
}
