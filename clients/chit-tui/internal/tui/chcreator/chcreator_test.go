package chcreator_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/infrashift/chit/clients/chit-tui/internal/model"
	"github.com/infrashift/chit/clients/chit-tui/internal/testutil"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/chcreator"
)

func TestChCreator_OpenClose(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	if m.Visible() {
		t.Error("should not be visible initially")
	}

	m.Open("t1")
	if !m.Visible() {
		t.Error("should be visible after Open()")
	}

	m.Close()
	if m.Visible() {
		t.Error("should not be visible after Close()")
	}
}

func TestChCreator_EscapeCloses(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if m.Visible() {
		t.Error("escape should close creator")
	}
}

func TestChCreator_FieldNavigation(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Tab through all fields
	for range 4 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	}
	// Should wrap back to displayName
	view := m.View()
	if !strings.Contains(view, "Create Channel") {
		t.Errorf("expected creator title in view:\n%s", view)
	}

	// Shift+Tab goes backward
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	_ = m.View()
}

func TestChCreator_AutoSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Deployments", "deployments"},
		{"My Channel", "my-channel"},
		{"  Hello World  ", "hello-world"},
		{"A--B", "a-b"},
		{"123", "123"},
		{"a", "a"},
	}

	for _, tt := range tests {
		got := chcreator.Slug(tt.input)
		if got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestChCreator_AutoSlugWhileTyping(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type display name
	for _, r := range "My Channel" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	view := m.View()
	if !strings.Contains(view, "my-channel") {
		t.Errorf("expected auto-slug 'my-channel' in view:\n%s", view)
	}
}

func TestChCreator_ManualNameDisablesAutoSlug(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type display name
	for _, r := range "Hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Tab to name field (disables auto-slug)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Clear and type custom name
	for range 5 {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "custom" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Tab to type field, then back to display name
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Type more in display name — should NOT update channel name
	for _, r := range " World" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	view := m.View()
	if !strings.Contains(view, "custom") {
		t.Errorf("expected 'custom' name preserved in view:\n%s", view)
	}
}

func TestChCreator_TypeToggle(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Navigate to type field (2 tabs: displayName → name → type)
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Default is Open
	view := m.View()
	if !strings.Contains(view, "[Open]") {
		t.Errorf("expected [Open] selected:\n%s", view)
	}

	// Right arrow switches to Private
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})
	view = m.View()
	if !strings.Contains(view, "[Private]") {
		t.Errorf("expected [Private] selected:\n%s", view)
	}

	// Left arrow switches back to Open
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	view = m.View()
	if !strings.Contains(view, "[Open]") {
		t.Errorf("expected [Open] selected after left:\n%s", view)
	}
}

func TestChCreator_ValidationEmptyDisplayName(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Enter without typing anything
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command on validation failure")
	}
	view := m.View()
	if !strings.Contains(view, "Display name is required") {
		t.Errorf("expected validation error:\n%s", view)
	}
}

func TestChCreator_ValidationInvalidName(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type a display name that generates an invalid slug (single char)
	for _, r := range "X" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Auto-slug will be "x" which is only 1 char — invalid per regex (min 2)
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("expected nil command on validation failure")
	}
	view := m.View()
	if !strings.Contains(view, "Invalid name") {
		t.Errorf("expected invalid name error:\n%s", view)
	}
}

func TestChCreator_SubmitSuccess(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type display name
	for _, r := range "Deployments" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Submit
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from submit")
	}
	msg := cmd()
	submitted, ok := msg.(chcreator.ChannelSubmittedMsg)
	if !ok {
		t.Fatalf("expected ChannelSubmittedMsg, got %T", msg)
	}
	if submitted.Channel.DisplayName != "Deployments" {
		t.Errorf("display name = %q", submitted.Channel.DisplayName)
	}
	if submitted.Channel.Name != "deployments" {
		t.Errorf("name = %q", submitted.Channel.Name)
	}
	if submitted.Channel.Type != model.ChannelOpen {
		t.Errorf("type = %q, want O", submitted.Channel.Type)
	}
	if submitted.Channel.TeamID != "t1" {
		t.Errorf("teamID = %q", submitted.Channel.TeamID)
	}
}

func TestChCreator_SubmitPrivate(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type display name
	for _, r := range "Secret Ops" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Tab to name, tab to type
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	// Switch to private
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRight})

	// Submit
	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from submit")
	}
	msg := cmd()
	submitted := msg.(chcreator.ChannelSubmittedMsg)
	if submitted.Channel.Type != model.ChannelPrivate {
		t.Errorf("type = %q, want P", submitted.Channel.Type)
	}
}

func TestChCreator_SubmitWithPurpose(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Type display name
	for _, r := range "Deployments" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Tab to name, type, purpose
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})

	for _, r := range "CI/CD notifications" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from submit")
	}
	msg := cmd()
	submitted := msg.(chcreator.ChannelSubmittedMsg)
	if submitted.Channel.Purpose != "CI/CD notifications" {
		t.Errorf("purpose = %q", submitted.Channel.Purpose)
	}
}

func TestChCreator_ViewHiddenEmpty(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)

	view := m.View()
	if view != "" {
		t.Errorf("expected empty view when not visible: %s", view)
	}
}

func TestChCreator_ViewShowsAllFields(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	view := m.View()
	if !strings.Contains(view, "Display Name") {
		t.Error("expected Display Name label")
	}
	if !strings.Contains(view, "Channel Name") {
		t.Error("expected Channel Name label")
	}
	if !strings.Contains(view, "Type") {
		t.Error("expected Type label")
	}
	if !strings.Contains(view, "Purpose") {
		t.Error("expected Purpose label")
	}
}

func TestChCreator_IgnoresInputWhenNotVisible(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)

	m, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Error("should not emit command when not visible")
	}
}

func TestChCreator_ValidateName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"ab", true},
		{"deploy", true},
		{"my-channel", true},
		{"a1b2", true},
		{"a", false},
		{"-bad", false},
		{"bad-", false},
		{"UPPER", false},
		{"has space", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := chcreator.ValidateName(tt.name); got != tt.valid {
			t.Errorf("ValidateName(%q) = %v, want %v", tt.name, got, tt.valid)
		}
	}
}

func TestChCreator_ClearsValidErrOnType(t *testing.T) {
	m := chcreator.New(testutil.Styles())
	m.SetSize(80, 40)
	m.Open("t1")

	// Trigger validation error
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view := m.View()
	if !strings.Contains(view, "required") {
		t.Error("expected validation error")
	}

	// Type a character — error should clear
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'D'}})
	view = m.View()
	if strings.Contains(view, "required") {
		t.Error("validation error should clear after typing")
	}
}

// The slug kept letters outside a-z, which the name check then rejected,
// and cut long names by byte, splitting a character.
func TestSlug_IsAlwaysAValidName(t *testing.T) {
	for _, in := range []string{
		"Café Ops",
		"日本語 team",
		strings.Repeat("é", 40) + " and more words to make it long",
		"Deploy -- Notes",
	} {
		got := chcreator.Slug(in)
		if got != "" && !chcreator.ValidateName(got) {
			t.Errorf("Slug(%q) = %q, which the name check rejects", in, got)
		}
		if !utf8.ValidString(got) {
			t.Errorf("Slug(%q) = %q, not valid UTF-8", in, got)
		}
	}
	if got := chcreator.Slug("Café Ops"); got != "caf-ops" && got != "cafe-ops" {
		t.Errorf(`Slug("Café Ops") = %q`, got)
	}
}
