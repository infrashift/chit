package markdown

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit-tui/internal/tui/ui/theme"
)

// StyleConfig builds a glamour ansi.StyleConfig derived from the active theme
// so that all markdown elements render in colors matching the selected theme.
func StyleConfig(t theme.Theme) ansi.StyleConfig {
	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockPrefix: "\n",
				BlockSuffix: "\n",
				Color:       colorStr(t.Foreground),
			},
			Margin: uintPtr(0),
		},
		BlockQuote: ansi.StyleBlock{
			Indent:      uintPtr(1),
			IndentToken: stringPtr("│ "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				BlockSuffix: "\n",
				Color:       colorStr(t.Username),
				Bold:        boolPtr(true),
			},
		},
		H1: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "# ",
				Color:  colorStr(t.Username),
				Bold:   boolPtr(true),
			},
		},
		H2: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "## ",
				Color:  colorStr(t.Warning),
				Bold:   boolPtr(true),
			},
		},
		H3: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "### ",
				Color:  colorStr(t.Success),
				Bold:   boolPtr(true),
			},
		},
		H4: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "#### ",
				Color:  colorStr(t.Accent),
				Bold:   boolPtr(true),
			},
		},
		H5: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "##### ",
				Color:  colorStr(t.Subtle),
				Bold:   boolPtr(true),
			},
		},
		H6: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix: "###### ",
				Color:  colorStr(t.Muted),
				Bold:   boolPtr(false),
			},
		},
		Strikethrough: ansi.StylePrimitive{
			CrossedOut: boolPtr(true),
		},
		Emph: ansi.StylePrimitive{
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Bold: boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  colorStr(t.Subtle),
			Format: "\n--------\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       colorStr(t.Accent),
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     colorStr(t.Accent),
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: colorStr(t.Accent),
			Bold:  boolPtr(true),
		},
		Image: ansi.StylePrimitive{
			Color:     colorStr(t.Accent),
			Underline: boolPtr(true),
		},
		ImageText: ansi.StylePrimitive{
			Color:  colorStr(t.Subtle),
			Format: "Image: {{.text}} →",
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Prefix:          " ",
				Suffix:          " ",
				Color:           colorStr(t.Success),
				BackgroundColor: colorStr(t.Highlight),
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color: colorStr(t.Foreground),
				},
				Margin: uintPtr(2),
			},
			Chroma: &ansi.Chroma{
				Text: ansi.StylePrimitive{
					Color: colorStr(t.Foreground),
				},
				Error: ansi.StylePrimitive{
					Color: colorStr(t.Error),
				},
				Comment: ansi.StylePrimitive{
					Color: colorStr(t.Subtle),
				},
				CommentPreproc: ansi.StylePrimitive{
					Color: colorStr(t.Warning),
				},
				Keyword: ansi.StylePrimitive{
					Color: colorStr(t.Accent),
				},
				KeywordReserved: ansi.StylePrimitive{
					Color: colorStr(t.Accent),
				},
				KeywordNamespace: ansi.StylePrimitive{
					Color: colorStr(t.Accent),
				},
				KeywordType: ansi.StylePrimitive{
					Color: colorStr(t.Username),
				},
				Operator: ansi.StylePrimitive{
					Color: colorStr(t.Error),
				},
				Punctuation: ansi.StylePrimitive{
					Color: colorStr(t.Foreground),
				},
				Name: ansi.StylePrimitive{
					Color: colorStr(t.Foreground),
				},
				NameBuiltin: ansi.StylePrimitive{
					Color: colorStr(t.Accent),
				},
				NameTag: ansi.StylePrimitive{
					Color: colorStr(t.Username),
				},
				NameAttribute: ansi.StylePrimitive{
					Color: colorStr(t.Accent),
				},
				NameClass: ansi.StylePrimitive{
					Color:     colorStr(t.Username),
					Underline: boolPtr(true),
					Bold:      boolPtr(true),
				},
				NameDecorator: ansi.StylePrimitive{
					Color: colorStr(t.Warning),
				},
				NameFunction: ansi.StylePrimitive{
					Color: colorStr(t.Success),
				},
				LiteralNumber: ansi.StylePrimitive{
					Color: colorStr(t.Success),
				},
				LiteralString: ansi.StylePrimitive{
					Color: colorStr(t.Warning),
				},
				LiteralStringEscape: ansi.StylePrimitive{
					Color: colorStr(t.Success),
				},
				GenericDeleted: ansi.StylePrimitive{
					Color: colorStr(t.Error),
				},
				GenericEmph: ansi.StylePrimitive{
					Italic: boolPtr(true),
				},
				GenericInserted: ansi.StylePrimitive{
					Color: colorStr(t.Success),
				},
				GenericStrong: ansi.StylePrimitive{
					Bold: boolPtr(true),
				},
				GenericSubheading: ansi.StylePrimitive{
					Color: colorStr(t.Subtle),
				},
				Background: ansi.StylePrimitive{
					BackgroundColor: colorStr(t.Background),
				},
			},
		},
		Table: ansi.StyleTable{},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n🠶 ",
		},
	}
}

func colorStr(c lipgloss.Color) *string {
	s := string(c)
	return &s
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }
