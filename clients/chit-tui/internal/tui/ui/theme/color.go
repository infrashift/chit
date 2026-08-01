package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Colors are image/color.Color rather than lipgloss.Color so palettes can be
// derived arithmetically (see ShiftLightness and Blend). A nil color means
// "terminal default" — Lip turns it into lipgloss.NoColor, and the fg/bg
// helpers in styles.go skip the attribute entirely.

// rgbComponents extracts 8-bit RGB from a color. It reports ok=false for nil
// and for colors with no RGB representation (ANSI palette indices), which
// callers use to pass such colors through unmodified.
func rgbComponents(c color.Color) (r, g, b uint8, ok bool) {
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
	case lipgloss.Color:
		parsed := hexColor(string(v))
		if parsed == nil {
			return 0, 0, 0, false
		}
		return rgbComponents(parsed)
	}

	// color.Color returns 16-bit alpha-premultiplied values.
	r16, g16, b16, a16 := c.RGBA()
	if a16 == 0 {
		return 0, 0, 0, false
	}
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

// ShiftLightness moves every channel by amount: up when the channel average is
// below 128 (dark colors get lighter) and down otherwise. This is an exact port
// of tuicr's shift_lightness — deliberately not an HSL-based lighten/darken,
// which produces visibly different results on desaturated colors.
func ShiftLightness(c color.Color, amount int) color.Color {
	r, g, b, ok := rgbComponents(c)
	if !ok {
		return c
	}

	avg := (int(r) + int(g) + int(b)) / 3
	delta := amount
	if avg >= 128 {
		delta = -amount
	}

	return color.RGBA{
		R: clamp8(int(r) + delta),
		G: clamp8(int(g) + delta),
		B: clamp8(int(b) + delta),
		A: 0xff,
	}
}

// Blend mixes accent into base by accentPercent (0-100). It is how the palette
// generators derive highlight and badge backgrounds from a base palette.
func Blend(base, accent color.Color, accentPercent int) color.Color {
	br, bg, bb, baseOK := rgbComponents(base)
	ar, ag, ab, accentOK := rgbComponents(accent)

	switch {
	case !accentOK:
		return base
	case !baseOK:
		return accent
	}

	if accentPercent < 0 {
		accentPercent = 0
	}
	if accentPercent > 100 {
		accentPercent = 100
	}

	mix := func(b, a uint8) uint8 {
		return clamp8((int(b)*(100-accentPercent) + int(a)*accentPercent) / 100)
	}

	return color.RGBA{R: mix(br, ar), G: mix(bg, ag), B: mix(bb, ab), A: 0xff}
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
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
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
	return lipgloss.Color(hex)
}
