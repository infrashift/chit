package markdown

import (
	"image/color"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/tui/ui/theme"
)

// StyleConfig builds a glamour ansi.StyleConfig derived from the active theme
// so that all markdown elements render in colors matching the selected theme.
func StyleConfig(t theme.Theme) ansi.StyleConfig {
	cfg := ansi.StyleConfig{
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
			Chroma: chroma(t),
		},
		Table: ansi.StyleTable{},
		DefinitionDescription: ansi.StylePrimitive{
			BlockPrefix: "\n🠶 ",
		},
	}
	setHeadings(&cfg, t)
	setInlines(&cfg, t)
	setLists(&cfg, t)
	return cfg
}

// heading styles a heading level: its marker, color and weight.
func heading(prefix string, c color.Color, bold bool) ansi.StyleBlock {
	return ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: prefix,
			Color:  colorStr(c),
			Bold:   boolPtr(bold),
		},
	}
}

// setHeadings styles the headings, each level in its own color.
func setHeadings(cfg *ansi.StyleConfig, t theme.Theme) {
	cfg.Heading = ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			BlockSuffix: "\n",
			Color:       colorStr(t.Username),
			Bold:        boolPtr(true),
		},
	}
	cfg.H1 = heading("# ", t.Username, true)
	cfg.H2 = heading("## ", t.Warning, true)
	cfg.H3 = heading("### ", t.Success, true)
	cfg.H4 = heading("#### ", t.Accent, true)
	cfg.H5 = heading("##### ", t.Subtle, true)
	cfg.H6 = heading("###### ", t.Muted, false)
}

// setInlines styles emphasis, rules, links and images.
func setInlines(cfg *ansi.StyleConfig, t theme.Theme) {
	cfg.Strikethrough = ansi.StylePrimitive{
		CrossedOut: boolPtr(true),
	}
	cfg.Emph = ansi.StylePrimitive{
		Italic: boolPtr(true),
	}
	cfg.Strong = ansi.StylePrimitive{
		Bold: boolPtr(true),
	}
	cfg.HorizontalRule = ansi.StylePrimitive{
		Color:  colorStr(t.Subtle),
		Format: "\n--------\n",
	}
	cfg.Link = ansi.StylePrimitive{
		Color:     colorStr(t.Accent),
		Underline: boolPtr(true),
	}
	cfg.LinkText = ansi.StylePrimitive{
		Color: colorStr(t.Accent),
		Bold:  boolPtr(true),
	}
	cfg.Image = ansi.StylePrimitive{
		Color:     colorStr(t.Accent),
		Underline: boolPtr(true),
	}
	cfg.ImageText = ansi.StylePrimitive{
		Color:  colorStr(t.Subtle),
		Format: "Image: {{.text}} →",
	}
}

// setLists styles list items, numbering and task boxes.
func setLists(cfg *ansi.StyleConfig, t theme.Theme) {
	cfg.Item = ansi.StylePrimitive{
		BlockPrefix: "• ",
	}
	cfg.Enumeration = ansi.StylePrimitive{
		BlockPrefix: ". ",
		Color:       colorStr(t.Accent),
	}
	cfg.Task = ansi.StyleTask{
		Ticked:   "[✓] ",
		Unticked: "[ ] ",
	}
}

// fgColor is a primitive that sets only the foreground color.
func fgColor(c color.Color) ansi.StylePrimitive {
	return ansi.StylePrimitive{Color: colorStr(c)}
}

// chroma styles syntax highlighting in code blocks.
func chroma(t theme.Theme) *ansi.Chroma {
	c := &ansi.Chroma{
		Text:             fgColor(t.Foreground),
		Error:            fgColor(t.Error),
		Comment:          fgColor(t.Subtle),
		CommentPreproc:   fgColor(t.Warning),
		Keyword:          fgColor(t.Accent),
		KeywordReserved:  fgColor(t.Accent),
		KeywordNamespace: fgColor(t.Accent),
		KeywordType:      fgColor(t.Username),
		Operator:         fgColor(t.Error),
		Punctuation:      fgColor(t.Foreground),
		Background: ansi.StylePrimitive{
			BackgroundColor: colorStr(t.Background),
		},
	}
	setChromaNames(c, t)
	setChromaLiterals(c, t)
	return c
}

// setChromaNames styles identifiers: names, tags, classes and functions.
func setChromaNames(c *ansi.Chroma, t theme.Theme) {
	c.Name = fgColor(t.Foreground)
	c.NameBuiltin = fgColor(t.Accent)
	c.NameTag = fgColor(t.Username)
	c.NameAttribute = fgColor(t.Accent)
	c.NameClass = ansi.StylePrimitive{
		Color:     colorStr(t.Username),
		Underline: boolPtr(true),
		Bold:      boolPtr(true),
	}
	c.NameDecorator = fgColor(t.Warning)
	c.NameFunction = fgColor(t.Success)
}

// setChromaLiterals styles literals and diff-style generic tokens.
func setChromaLiterals(c *ansi.Chroma, t theme.Theme) {
	c.LiteralNumber = fgColor(t.Success)
	c.LiteralString = fgColor(t.Warning)
	c.LiteralStringEscape = fgColor(t.Success)
	c.GenericDeleted = fgColor(t.Error)
	c.GenericEmph = ansi.StylePrimitive{
		Italic: boolPtr(true),
	}
	c.GenericInserted = fgColor(t.Success)
	c.GenericStrong = ansi.StylePrimitive{
		Bold: boolPtr(true),
	}
	c.GenericSubheading = fgColor(t.Subtle)
}

// colorStr renders a theme color as the hex string glamour expects. Colors with
// no RGB form (nil, ANSI indices) yield an empty string, which glamour treats as
// "inherit" rather than painting a color.
func colorStr(c color.Color) *string {
	s := theme.Hex(c)
	return &s
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }
