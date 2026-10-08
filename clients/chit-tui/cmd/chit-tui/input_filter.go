package main

import tea "github.com/charmbracelet/bubbletea"

// filterInput adjusts keys before the model sees them.
//
// Terminal response sequences (OSC replies, cursor position reports) that
// leak into the input as runes are dropped, or they would be typed as
// garbage.
//
// A key holding several runes is text arriving in one burst: typed fast over
// a slow link, or sent without bracketed paste. Bubble Tea splits such a
// burst at spaces, and text inputs match keys by name, so a chunk that is
// exactly "right" moved the cursor instead of being typed. Named keys never
// arrive as runes, so these are marked as pasted, which no binding matches.
func filterInput(msg tea.Msg) tea.Msg {
	k, ok := msg.(tea.KeyMsg)
	if !ok || k.Type != tea.KeyRunes {
		return msg
	}
	if termResponseRe.MatchString(string(k.Runes)) {
		return nil
	}
	if len(k.Runes) > 1 && !k.Paste {
		k.Paste = true
		return k
	}
	return msg
}
