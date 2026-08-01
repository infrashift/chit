package markdown_test

import (
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/markdown"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

func TestStyleConfig_DocumentMarginZero(t *testing.T) {
	sc := markdown.StyleConfig(theme.TokyoNight())
	if sc.Document.Margin == nil || *sc.Document.Margin != 0 {
		t.Error("expected Document.Margin == 0")
	}
}

func TestStyleConfig_HeadingColors(t *testing.T) {
	th := theme.TokyoNight()
	sc := markdown.StyleConfig(th)

	tests := []struct {
		name  string
		color *string
		want  string
	}{
		{"H1", sc.H1.Color, theme.Hex(th.Username)},
		{"H2", sc.H2.Color, theme.Hex(th.Warning)},
		{"H3", sc.H3.Color, theme.Hex(th.Success)},
		{"H4", sc.H4.Color, theme.Hex(th.Accent)},
		{"H5", sc.H5.Color, theme.Hex(th.Subtle)},
		{"H6", sc.H6.Color, theme.Hex(th.Muted)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.color == nil {
				t.Fatalf("%s color is nil", tt.name)
			}
			if *tt.color != tt.want {
				t.Errorf("%s color = %q, want %q", tt.name, *tt.color, tt.want)
			}
		})
	}
}

func TestStyleConfig_LinkStyle(t *testing.T) {
	th := theme.TokyoNight()
	sc := markdown.StyleConfig(th)

	if sc.Link.Color == nil || *sc.Link.Color != theme.Hex(th.Accent) {
		t.Errorf("Link.Color = %v, want %q", sc.Link.Color, theme.Hex(th.Accent))
	}
	if sc.Link.Underline == nil || !*sc.Link.Underline {
		t.Error("Link.Underline should be true")
	}
}

func TestStyleConfig_CodeStyle(t *testing.T) {
	th := theme.TokyoNight()
	sc := markdown.StyleConfig(th)

	if sc.Code.Color == nil || *sc.Code.Color != theme.Hex(th.Success) {
		t.Errorf("Code.Color = %v, want %q", sc.Code.Color, theme.Hex(th.Success))
	}
	if sc.Code.BackgroundColor == nil || *sc.Code.BackgroundColor != theme.Hex(th.Highlight) {
		t.Errorf("Code.BackgroundColor = %v, want %q", sc.Code.BackgroundColor, theme.Hex(th.Highlight))
	}
}

func TestStyleConfig_AllBuiltinThemes(t *testing.T) {
	themes := []struct {
		name string
		fn   func() theme.Theme
	}{
		{"TokyoNight", theme.TokyoNight},
		{"Catppuccin", theme.Catppuccin},
		{"Kanagawa", theme.Kanagawa},
		{"Nightfox", theme.Nightfox},
	}

	for _, tt := range themes {
		t.Run(tt.name, func(t *testing.T) {
			sc := markdown.StyleConfig(tt.fn())
			if sc.Document.Color == nil {
				t.Error("Document.Color should not be nil")
			}
			if sc.Heading.Color == nil {
				t.Error("Heading.Color should not be nil")
			}
			if sc.Link.Color == nil {
				t.Error("Link.Color should not be nil")
			}
			if sc.Code.Color == nil {
				t.Error("Code.Color should not be nil")
			}
			if sc.CodeBlock.Chroma == nil {
				t.Error("CodeBlock.Chroma should not be nil")
			}
		})
	}
}
