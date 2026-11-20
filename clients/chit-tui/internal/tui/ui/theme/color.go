package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/infrashift/chit/clients/chit-tui/internal/assert"
)

// Colors are image/color.Color rather than lipgloss.Color so palettes can be
// derived arithmetically (see Blend). A nil color means
// "terminal default" — Lip turns it into lipgloss.NoColor, and the fg/bg
// helpers in styles.go skip the attribute entirely.

// rgbComponents extracts 8-bit RGB from a color. It reports ok=false for nil
// and for colors with no RGB representation (ANSI palette indices), which
// callers use to pass such colors through unmodified.
func rgbComponents(c color.Color) (r, g, b uint8, ok bool) {
	// A lipgloss.Color holds a hex string. Parse it first, so the switch
	// below sees a color.RGBA (Power of 10 rule 1: no recursion).
	if lc, isHex := c.(lipgloss.Color); isHex {
		c = hexColor(string(lc))
	}
	_, stillHex := c.(lipgloss.Color)
	assert.That(!stillHex, "rgbComponents: lipgloss.Color left unparsed")
	if c == nil {
		return 0, 0, 0, false
	}

	switch v := c.(type) {
	case color.RGBA:
		// A fully transparent color has no renderable RGB, same as the
		// generic path below; report it as unset rather than black.
		if v.A == 0 {
			return 0, 0, 0, false
		}
		return v.R, v.G, v.B, true
	case color.NRGBA:
		if v.A == 0 {
			return 0, 0, 0, false
		}
		return v.R, v.G, v.B, true
	}

	// color.Color returns 16-bit alpha-premultiplied values.
	r16, g16, b16, a16 := c.RGBA()
	if a16 == 0 {
		return 0, 0, 0, false
	}
	assert.That(r16 <= a16 && g16 <= a16 && b16 <= a16, "rgbComponents: premultiplied component above alpha")
	return uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8), true
}

// clamp8 constrains an int to the 0-255 range.
func clamp8(v int) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return uint8(v)
	}
}

// Blend mixes accent into base by accentPercent (0-100). It is how the palette
// generators derive highlight and badge backgrounds from a base palette.
func Blend(base, accent color.Color, accentPercent int) color.Color {
	br, bg, bb, baseOK := rgbComponents(base)
	ar, ag, ab, accentOK := rgbComponents(accent)

	// A terminal color name has no RGB value to mix. Falling back to the
	// base made a derived highlight the background color itself, so it was
	// invisible; the accent at least stays visible.
	switch {
	case accent == nil:
		return base
	case !accentOK || !baseOK:
		return accent
	}

	if accentPercent < 0 {
		accentPercent = 0
	}
	if accentPercent > 100 {
		accentPercent = 100
	}
	assert.That(accentPercent >= 0 && accentPercent <= 100, "Blend: percent outside 0-100 after clamping")

	mix := func(b, a uint8) uint8 {
		return clamp8((int(b)*(100-accentPercent) + int(a)*accentPercent) / 100)
	}

	out := color.RGBA{R: mix(br, ar), G: mix(bg, ag), B: mix(bb, ab), A: 0xff}
	assert.That(out.R >= min(br, ar) && out.R <= max(br, ar), "Blend: red channel outside the two inputs")
	assert.That(out.G >= min(bg, ag) && out.G <= max(bg, ag), "Blend: green channel outside the two inputs")
	assert.That(out.B >= min(bb, ab) && out.B <= max(bb, ab), "Blend: blue channel outside the two inputs")
	return out
}

// hexColor parses "#rgb" and "#rrggbb". Anything else yields nil, which callers
// treat as "unset" rather than an error.
func hexColor(s string) color.Color {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return nil
	}
	digits := s[1:]

	switch len(digits) {
	case 3:
		var expanded strings.Builder
		for _, r := range digits {
			expanded.WriteRune(r)
			expanded.WriteRune(r)
		}
		digits = expanded.String()
	case 6:
	default:
		return nil
	}

	v, err := strconv.ParseUint(digits, 16, 32)
	if err != nil {
		return nil
	}
	assert.That(len(digits) == 6, "hexColor: parsed digits not six long")
	assert.That(v <= 0xffffff, "hexColor: value wider than 24 bits")

	return color.RGBA{
		R: uint8(v >> 16),
		G: uint8(v >> 8),
		B: uint8(v),
		A: 0xff,
	}
}

// colorToHex renders a color as "#rrggbb", or "" when it has no RGB form.
func colorToHex(c color.Color) string {
	r, g, b, ok := rgbComponents(c)
	if !ok {
		return ""
	}
	hex := fmt.Sprintf("#%02x%02x%02x", r, g, b)
	assert.That(len(hex) == 7, "colorToHex: hex not seven characters")
	assert.That(hex[0] == '#', "colorToHex: hex lacks the # prefix")
	return hex
}

// brighter reports whether a is brighter than b, comparing the sum of the
// RGB channels. Either color lacking an RGB form compares as not brighter.
func brighter(a, b color.Color) bool {
	ar, ag, ab, aOK := rgbComponents(a)
	br, bg, bb, bOK := rgbComponents(b)
	if !aOK || !bOK {
		return false
	}
	return int(ar)+int(ag)+int(ab) > int(br)+int(bg)+int(bb)
}

// Hex renders a color as "#rrggbb" for consumers that need a string, such as
// glamour's style config. Colors with no RGB form yield "".
func Hex(c color.Color) string { return colorToHex(c) }

// Lip adapts a color.Color to lipgloss v1, whose Foreground and Background only
// accept lipgloss.TerminalColor. Colors that carry no RGB form (ANSI indices)
// pass through as-is; nil becomes NoColor so the terminal default shows.
func Lip(c color.Color) lipgloss.TerminalColor {
	if c == nil {
		return lipgloss.NoColor{}
	}
	if lc, ok := c.(lipgloss.Color); ok {
		return lc
	}
	hex := colorToHex(c)
	if hex == "" {
		return lipgloss.NoColor{}
	}
	assert.That(len(hex) == 7 && hex[0] == '#', "Lip: hex not #rrggbb")
	return lipgloss.Color(hex)
}
