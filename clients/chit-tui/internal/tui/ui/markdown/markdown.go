package markdown

import (
	"image/color"
	"unicode/utf8"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
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
	assert.That(cfg.Document.Margin != nil, "StyleConfig: document margin unset")
	assert.That(cfg.Document.Color != nil, "StyleConfig: document color unset")
	assert.That(cfg.CodeBlock.Chroma != nil, "StyleConfig: code block has no chroma styles")
	assert.That(cfg.BlockQuote.Indent != nil && *cfg.BlockQuote.Indent > 0, "StyleConfig: block quote not indented")
	assert.That(cfg.H1.Prefix != "", "StyleConfig: H1 prefix unset")
	assert.That(cfg.H6.Prefix != "", "StyleConfig: H6 prefix unset")
	assert.That(cfg.Item.BlockPrefix != "", "StyleConfig: bullet marker unset")
	assert.That(cfg.Task.Ticked != "", "StyleConfig: task box unset")
	return cfg
}

// heading styles a heading level: its marker, color and weight.
func heading(prefix string, c color.Color, bold bool) ansi.StyleBlock {
	assert.That(prefix != "", "heading: empty prefix")
	assert.That(prefix[len(prefix)-1] == ' ', "heading: prefix must end in a space")
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
	assert.That(cfg != nil, "setHeadings: nil config")
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
	assert.That(cfg.Heading.Bold != nil && *cfg.Heading.Bold, "setHeadings: base heading not bold")
	assert.That(len(cfg.H1.Prefix) < len(cfg.H6.Prefix), "setHeadings: H1 marker not shorter than H6")
	assert.That(cfg.H5.Bold != nil && *cfg.H5.Bold, "setHeadings: H5 not bold")
	assert.That(cfg.H6.Bold != nil && !*cfg.H6.Bold, "setHeadings: H6 bold")
}

// setInlines styles emphasis, rules, links and images.
func setInlines(cfg *ansi.StyleConfig, t theme.Theme) {
	assert.That(cfg != nil, "setInlines: nil config")
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
	assert.That(cfg.Link.Color != nil, "setInlines: link color unset")
	assert.That(cfg.LinkText.Color != nil, "setInlines: link text color unset")
	assert.That(cfg.Emph.Italic != nil, "setInlines: italic emphasis unset")
	assert.That(cfg.Strong.Bold != nil, "setInlines: strong emphasis unset")
	assert.That(cfg.HorizontalRule.Format != "", "setInlines: rule format unset")
	assert.That(cfg.ImageText.Format != "", "setInlines: image text format unset")
}

// setLists styles list items, numbering and task boxes.
func setLists(cfg *ansi.StyleConfig, t theme.Theme) {
	assert.That(cfg != nil, "setLists: nil config")
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
	assert.That(cfg.Enumeration.Color != nil, "setLists: enumeration color unset")
	assert.That(cfg.Item.BlockPrefix != "", "setLists: bullet marker unset")
	assert.That(cfg.Enumeration.BlockPrefix != "", "setLists: enumeration marker unset")
	assert.That(utf8.RuneCountInString(cfg.Task.Ticked) == utf8.RuneCountInString(cfg.Task.Unticked), "setLists: task boxes differ in width")
}

// fgColor is a primitive that sets only the foreground color.
func fgColor(c color.Color) ansi.StylePrimitive {
	p := ansi.StylePrimitive{Color: colorStr(c)}
	assert.That(p.Color != nil, "fgColor: color pointer unset")
	assert.That(p.BackgroundColor == nil, "fgColor: background set on a foreground-only primitive")
	return p
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
	assert.That(c.Text.Color != nil, "chroma: text color unset")
	assert.That(c.Background.BackgroundColor != nil, "chroma: background color unset")
	assert.That(c.Name.Color != nil, "chroma: name color unset")
	assert.That(c.LiteralString.Color != nil, "chroma: string literal color unset")
	assert.That(c.Keyword.Color != nil, "chroma: keyword color unset")
	assert.That(c.Comment.Color != nil, "chroma: comment color unset")
	return c
}

// setChromaNames styles identifiers: names, tags, classes and functions.
func setChromaNames(c *ansi.Chroma, t theme.Theme) {
	assert.That(c != nil, "setChromaNames: nil chroma")
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
	assert.That(c.NameClass.Color != nil && c.NameClass.Bold != nil, "setChromaNames: class style unset")
	assert.That(c.NameBuiltin.Color != nil, "setChromaNames: builtin color unset")
	assert.That(c.NameFunction.Color != nil, "setChromaNames: function color unset")
}

// setChromaLiterals styles literals and diff-style generic tokens.
func setChromaLiterals(c *ansi.Chroma, t theme.Theme) {
	assert.That(c != nil, "setChromaLiterals: nil chroma")
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
	assert.That(c.GenericEmph.Italic != nil, "setChromaLiterals: generic italic unset")
	assert.That(c.GenericStrong.Bold != nil, "setChromaLiterals: generic bold unset")
	assert.That(c.LiteralNumber.Color != nil, "setChromaLiterals: number color unset")
	assert.That(c.GenericDeleted.Color != nil, "setChromaLiterals: deleted-line color unset")
}

// colorStr renders a theme color as the hex string glamour expects. Colors with
// no RGB form (nil, ANSI indices) yield an empty string, which glamour treats as
// "inherit" rather than painting a color.
func colorStr(c color.Color) *string {
	s := theme.Hex(c)
	assert.That(s == "" || (len(s) == 7 && s[0] == '#'), "colorStr: hex not empty or #rrggbb")
	assert.That(c != nil || s == "", "colorStr: hex for a nil color")
	return &s
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }
