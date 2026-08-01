package styles

import (
	"image/color"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/markdown"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// fg, bg, and border apply a theme color only when it is set. A nil slot leaves
// the attribute off entirely so the terminal's own default shows through, which
// is what makes transparent-background themes work.

func fg(s lipgloss.Style, c color.Color) lipgloss.Style {
	if c == nil {
		return s
	}
	return s.Foreground(theme.Lip(c))
}

func bg(s lipgloss.Style, c color.Color) lipgloss.Style {
	if c == nil {
		return s
	}
	return s.Background(theme.Lip(c))
}

func border(s lipgloss.Style, c color.Color) lipgloss.Style {
	if c == nil {
		return s
	}
	return s.BorderForeground(theme.Lip(c))
}

// Styles holds all lipgloss styles for the TUI.
type Styles struct {
	ListItem             lipgloss.Style
	ListItemActive       lipgloss.Style
	Viewport             lipgloss.Style
	Input                lipgloss.Style
	ThreadPanel          lipgloss.Style
	CmdPalette           lipgloss.Style
	StatusBar            lipgloss.Style
	BarButton            lipgloss.Style
	Username             lipgloss.Style
	Timestamp            lipgloss.Style
	UnreadBadge          lipgloss.Style
	PinBadge             lipgloss.Style
	ErrorText            lipgloss.Style
	ActiveBorder         lipgloss.Style
	MentionBadge         lipgloss.Style
	MentionText          lipgloss.Style
	MentionSelfHighlight lipgloss.Style
	AutocompletePanel    lipgloss.Style
	AutocompleteItem     lipgloss.Style
	AutocompleteActive   lipgloss.Style
	SelectedPost         lipgloss.Style
	ReplyIndent          lipgloss.Style
	TagBadge             lipgloss.Style
	DaySeparator         lipgloss.Style
	Selection            lipgloss.Style
	SearchMatch          lipgloss.Style
	SearchMatchActive    lipgloss.Style
	MarkdownStyleConfig  ansi.StyleConfig
}

// New creates Styles from a Theme.
func New(t theme.Theme) Styles {
	base := lipgloss.NewStyle()
	rounded := base.BorderStyle(lipgloss.RoundedBorder())
	double := base.BorderStyle(lipgloss.DoubleBorder())

	return Styles{
		ListItem: fg(base, t.Foreground).
			Padding(0, 1),

		ListItemActive: fg(base, t.ChannelActive).
			Bold(true).
			Padding(0, 1),

		Viewport: border(rounded, t.Border).
			Padding(0, 1),

		Input: border(rounded, t.Border).
			Padding(0, 1),

		ThreadPanel: border(rounded, t.Border).
			Padding(0, 1),

		CmdPalette: border(double, t.Accent).
			Padding(1, 2),

		StatusBar: fg(bg(base, t.Highlight), t.Subtle).
			Padding(0, 1),

		BarButton: fg(bg(base, t.Highlight), t.Accent).
			Bold(true),

		Username: fg(base, t.Username).
			Bold(true),

		Timestamp: fg(base, t.Timestamp),

		UnreadBadge: fg(base, t.UnreadBadge).
			Bold(true),

		PinBadge: fg(base, t.PinBadge),

		ErrorText: fg(base, t.Error),

		ActiveBorder: border(rounded, t.ActiveBorder),

		MentionBadge: fg(base, t.MentionBadge).
			Bold(true),

		MentionText: fg(base, t.MentionText).
			Bold(true),

		MentionSelfHighlight: fg(bg(base, t.MentionSelfBg), t.Foreground).
			Bold(true),

		AutocompletePanel: border(double, t.Accent).
			Padding(0, 1),

		AutocompleteItem: fg(base, t.Foreground),

		AutocompleteActive: fg(base, t.ChannelActive).
			Bold(true),

		SelectedPost: bg(base, t.Highlight).
			Padding(0, 1),

		ReplyIndent: border(base.
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderTop(false).
			BorderRight(false).
			BorderBottom(false), t.Subtle).
			PaddingLeft(1),

		TagBadge: fg(base, t.TagBadge).
			Bold(true),

		DaySeparator: fg(base, t.Muted),

		Selection: bg(base, t.Selection),

		SearchMatch: bg(base, t.SearchMatch),

		SearchMatchActive: fg(bg(base, t.SearchMatchActive), t.Foreground).
			Bold(true),

		MarkdownStyleConfig: markdown.StyleConfig(t),
	}
}
