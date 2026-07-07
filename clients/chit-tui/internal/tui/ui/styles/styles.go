package styles

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/markdown"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// Styles holds all lipgloss styles for the TUI.
type Styles struct {
	Sidebar              lipgloss.Style
	SidebarItem          lipgloss.Style
	SidebarActive        lipgloss.Style
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
	Border               lipgloss.Style
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
	MarkdownStyleConfig  ansi.StyleConfig
}

// New creates Styles from a Theme.
func New(t theme.Theme) Styles {
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)

	activeBorder := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(t.ActiveBorder)

	return Styles{
		Sidebar: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		SidebarItem: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Padding(0, 1),

		SidebarActive: lipgloss.NewStyle().
			Foreground(t.ChannelActive).
			Bold(true).
			Padding(0, 1),

		Viewport: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		Input: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		ThreadPanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(t.Border).
			Padding(0, 1),

		CmdPalette: lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(t.Accent).
			Padding(1, 2),

		StatusBar: lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Subtle).
			Padding(0, 1),

		BarButton: lipgloss.NewStyle().
			Background(t.Highlight).
			Foreground(t.Accent).
			Bold(true),

		Username: lipgloss.NewStyle().
			Foreground(t.Username).
			Bold(true),

		Timestamp: lipgloss.NewStyle().
			Foreground(t.Timestamp),

		UnreadBadge: lipgloss.NewStyle().
			Foreground(t.UnreadBadge).
			Bold(true),

		PinBadge: lipgloss.NewStyle().
			Foreground(t.PinBadge),

		ErrorText: lipgloss.NewStyle().
			Foreground(t.Error),

		Border:       border,
		ActiveBorder: activeBorder,

		MentionBadge: lipgloss.NewStyle().
			Foreground(t.MentionBadge).
			Bold(true),

		MentionText: lipgloss.NewStyle().
			Foreground(t.MentionText).
			Bold(true),

		MentionSelfHighlight: lipgloss.NewStyle().
			Foreground(t.Foreground).
			Background(t.MentionSelfBg).
			Bold(true),

		AutocompletePanel: lipgloss.NewStyle().
			BorderStyle(lipgloss.DoubleBorder()).
			BorderForeground(t.Accent).
			Padding(0, 1),

		AutocompleteItem: lipgloss.NewStyle().
			Foreground(t.Foreground),

		AutocompleteActive: lipgloss.NewStyle().
			Foreground(t.ChannelActive).
			Bold(true),

		SelectedPost: lipgloss.NewStyle().
			Background(t.Highlight).
			Padding(0, 1),

		ReplyIndent: lipgloss.NewStyle().
			BorderStyle(lipgloss.ThickBorder()).
			BorderLeft(true).
			BorderTop(false).
			BorderRight(false).
			BorderBottom(false).
			BorderForeground(t.Subtle).
			PaddingLeft(1),

		TagBadge: lipgloss.NewStyle().
			Foreground(t.TagBadge).
			Bold(true),

		DaySeparator: lipgloss.NewStyle().
			Foreground(t.Muted),

		MarkdownStyleConfig: markdown.StyleConfig(t),
	}
}
