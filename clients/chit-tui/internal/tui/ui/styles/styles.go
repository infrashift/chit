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
	var s Styles
	s.setPanes(t)
	s.setText(t)
	s.setBadges(t)
	s.setHighlights(t)
	s.MarkdownStyleConfig = markdown.StyleConfig(t)
	return s
}

// setPanes styles the bordered panes and overlays and the bar.
func (s *Styles) setPanes(t theme.Theme) {
	base := lipgloss.NewStyle()
	rounded := base.BorderStyle(lipgloss.RoundedBorder())
	double := base.BorderStyle(lipgloss.DoubleBorder())

	s.Viewport = border(rounded, t.Border).
		Padding(0, 1)
	s.Input = border(rounded, t.Border).
		Padding(0, 1)
	s.ThreadPanel = border(rounded, t.Border).
		Padding(0, 1)
	s.CmdPalette = border(double, t.Accent).
		Padding(1, 2)
	s.StatusBar = fg(bg(base, t.Highlight), t.Subtle).
		Padding(0, 1)
	s.BarButton = fg(bg(base, t.Highlight), t.Accent).
		Bold(true)
	s.ActiveBorder = border(rounded, t.ActiveBorder)
	s.AutocompletePanel = border(double, t.Accent).
		Padding(0, 1)
	s.ReplyIndent = border(base.
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderTop(false).
		BorderRight(false).
		BorderBottom(false), t.Subtle).
		PaddingLeft(1)
}

// setText styles list items, names and other text.
func (s *Styles) setText(t theme.Theme) {
	base := lipgloss.NewStyle()

	s.ListItem = fg(base, t.Foreground).
		Padding(0, 1)
	s.ListItemActive = fg(base, t.ChannelActive).
		Bold(true).
		Padding(0, 1)
	s.Username = fg(base, t.Username).
		Bold(true)
	s.Timestamp = fg(base, t.Timestamp)
	s.ErrorText = fg(base, t.Error)
	s.AutocompleteItem = fg(base, t.Foreground)
	s.AutocompleteActive = fg(base, t.ChannelActive).
		Bold(true)
	s.DaySeparator = fg(base, t.Muted)
}

// setBadges styles the badges and mention markers.
func (s *Styles) setBadges(t theme.Theme) {
	base := lipgloss.NewStyle()

	s.UnreadBadge = fg(base, t.UnreadBadge).
		Bold(true)
	s.PinBadge = fg(base, t.PinBadge)
	s.MentionBadge = fg(base, t.MentionBadge).
		Bold(true)
	s.MentionText = fg(base, t.MentionText).
		Bold(true)
	s.MentionSelfHighlight = fg(bg(base, t.MentionSelfBg), t.Foreground).
		Bold(true)
	s.TagBadge = fg(base, t.TagBadge).
		Bold(true)
}

// setHighlights styles selections and search matches.
func (s *Styles) setHighlights(t theme.Theme) {
	base := lipgloss.NewStyle()

	s.SelectedPost = bg(base, t.Highlight).
		Padding(0, 1)
	s.Selection = bg(base, t.Selection)
	s.SearchMatch = bg(base, t.SearchMatch)
	s.SearchMatchActive = fg(bg(base, t.SearchMatchActive), t.Foreground).
		Bold(true)
}
