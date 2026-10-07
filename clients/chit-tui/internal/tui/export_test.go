package tui

// Focused reports which area has keyboard focus. Where keys land decides
// what single-letter keys do, so tests need to see it directly.
func (m Model) Focused() FocusArea { return m.focus }
