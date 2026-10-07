package theme

import (
	"image/color"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func rgb(t *testing.T, c color.Color) (uint8, uint8, uint8) {
	t.Helper()
	r, g, b, ok := rgbComponents(c)
	if !ok {
		t.Fatalf("rgbComponents(%v) not ok", c)
	}
	return r, g, b
}

func TestHexColor(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantNil bool
		r, g, b uint8
	}{
		{name: "six digit", in: "#7aa2f7", r: 0x7a, g: 0xa2, b: 0xf7},
		{name: "three digit expands", in: "#abc", r: 0xaa, g: 0xbb, b: 0xcc},
		{name: "uppercase", in: "#7AA2F7", r: 0x7a, g: 0xa2, b: 0xf7},
		{name: "missing hash", in: "7aa2f7", wantNil: true},
		{name: "bad length", in: "#7aa2f", wantNil: true},
		{name: "non hex", in: "#zzzzzz", wantNil: true},
		{name: "empty", in: "", wantNil: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := hexColor(tc.in)
			if tc.wantNil {
				if got != nil {
					t.Fatalf("hexColor(%q) = %v, want nil", tc.in, got)
				}
				return
			}
			r, g, b := rgb(t, got)
			if r != tc.r || g != tc.g || b != tc.b {
				t.Errorf("hexColor(%q) = %02x%02x%02x, want %02x%02x%02x",
					tc.in, r, g, b, tc.r, tc.g, tc.b)
			}
		})
	}
}

func TestColorToHex(t *testing.T) {
	if got := colorToHex(hexColor("#7aa2f7")); got != "#7aa2f7" {
		t.Errorf("colorToHex round-trip = %q, want #7aa2f7", got)
	}
	if got := colorToHex(nil); got != "" {
		t.Errorf("colorToHex(nil) = %q, want empty", got)
	}
}

func TestRGBComponentsRejectsNil(t *testing.T) {
	if _, _, _, ok := rgbComponents(nil); ok {
		t.Error("rgbComponents(nil) reported ok")
	}
}

func TestRGBComponentsAcceptsColorVariants(t *testing.T) {
	tests := []struct {
		name    string
		in      color.Color
		wantOK  bool
		r, g, b uint8
	}{
		{name: "RGBA", in: color.RGBA{R: 1, G: 2, B: 3, A: 0xff}, wantOK: true, r: 1, g: 2, b: 3},
		{name: "NRGBA", in: color.NRGBA{R: 4, G: 5, B: 6, A: 0xff}, wantOK: true, r: 4, g: 5, b: 6},
		{name: "lipgloss hex", in: lipgloss.Color("#010203"), wantOK: true, r: 1, g: 2, b: 3},
		{name: "lipgloss ansi index", in: lipgloss.Color("5"), wantOK: false},
		{name: "opaque gray falls back to RGBA()", in: color.Gray{Y: 0x80}, wantOK: true, r: 0x80, g: 0x80, b: 0x80},
		{name: "fully transparent", in: color.RGBA{R: 9, G: 9, B: 9, A: 0}, wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, g, b, ok := rgbComponents(tc.in)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if r != tc.r || g != tc.g || b != tc.b {
				t.Errorf("got %02x%02x%02x, want %02x%02x%02x", r, g, b, tc.r, tc.g, tc.b)
			}
		})
	}
}

func TestBlendEdgeCases(t *testing.T) {
	t.Run("nil accent returns base", func(t *testing.T) {
		r, _, _ := rgb(t, Blend(hexColor("#1a1b26"), nil, 50))
		if r != 0x1a {
			t.Errorf("got r=%02x, want base", r)
		}
	})

	t.Run("both nil stays nil", func(t *testing.T) {
		if Blend(nil, nil, 50) != nil {
			t.Error("want nil")
		}
	})

	t.Run("percent clamps below zero", func(t *testing.T) {
		r, _, _ := rgb(t, Blend(hexColor("#000000"), hexColor("#ffffff"), -20))
		if r != 0 {
			t.Errorf("got r=%d, want base after clamping to 0%%", r)
		}
	})

	t.Run("percent clamps above hundred", func(t *testing.T) {
		r, _, _ := rgb(t, Blend(hexColor("#000000"), hexColor("#ffffff"), 250))
		if r != 255 {
			t.Errorf("got r=%d, want accent after clamping to 100%%", r)
		}
	})
}

func TestLipRejectsUnrenderableColor(t *testing.T) {
	// A fully transparent color has no RGB form, so it must fall back to the
	// terminal default rather than painting black.
	if _, ok := Lip(color.RGBA{A: 0}).(lipgloss.NoColor); !ok {
		t.Errorf("Lip(transparent) = %T, want lipgloss.NoColor", Lip(color.RGBA{A: 0}))
	}
}

// ShiftLightness moves channels up when the channel average is below 128 and
// down otherwise. This is a port of tuicr's algorithm, not an HSL lighten.
func TestShiftLightness(t *testing.T) {
	t.Run("dark color gets lighter", func(t *testing.T) {
		r, g, b := rgb(t, ShiftLightness(hexColor("#1a1b26"), 18))
		if r != 0x1a+18 || g != 0x1b+18 || b != 0x26+18 {
			t.Errorf("got %02x%02x%02x, want each channel +18", r, g, b)
		}
	})

	t.Run("light color gets darker", func(t *testing.T) {
		r, g, b := rgb(t, ShiftLightness(hexColor("#c0caf5"), 18))
		if r != 0xc0-18 || g != 0xca-18 || b != 0xf5-18 {
			t.Errorf("got %02x%02x%02x, want each channel -18", r, g, b)
		}
	})

	t.Run("clamps at bounds", func(t *testing.T) {
		r, _, _ := rgb(t, ShiftLightness(hexColor("#000000"), 300))
		if r != 255 {
			t.Errorf("got r=%d, want clamp to 255", r)
		}
		r2, _, _ := rgb(t, ShiftLightness(hexColor("#ffffff"), 300))
		if r2 != 0 {
			t.Errorf("got r=%d, want clamp to 0", r2)
		}
	})

	t.Run("nil passes through", func(t *testing.T) {
		if ShiftLightness(nil, 18) != nil {
			t.Error("want nil")
		}
	})
}

func TestBlend(t *testing.T) {
	t.Run("zero percent keeps base", func(t *testing.T) {
		r, g, b := rgb(t, Blend(hexColor("#1a1b26"), hexColor("#9ece6a"), 0))
		if r != 0x1a || g != 0x1b || b != 0x26 {
			t.Errorf("got %02x%02x%02x, want base unchanged", r, g, b)
		}
	})

	t.Run("hundred percent is accent", func(t *testing.T) {
		r, g, b := rgb(t, Blend(hexColor("#1a1b26"), hexColor("#9ece6a"), 100))
		if r != 0x9e || g != 0xce || b != 0x6a {
			t.Errorf("got %02x%02x%02x, want accent", r, g, b)
		}
	})

	t.Run("midpoint averages", func(t *testing.T) {
		r, _, _ := rgb(t, Blend(hexColor("#000000"), hexColor("#c8c8c8"), 50))
		if r != 100 {
			t.Errorf("got r=%d, want 100", r)
		}
	})

	t.Run("nil base returns accent", func(t *testing.T) {
		r, _, _ := rgb(t, Blend(nil, hexColor("#9ece6a"), 20))
		if r != 0x9e {
			t.Errorf("got r=%02x, want accent", r)
		}
	})
}

// Lip bridges image/color.Color to lipgloss v1, whose Foreground/Background
// only accept lipgloss.TerminalColor. A nil color means "terminal default".
func TestLip(t *testing.T) {
	t.Run("nil becomes NoColor", func(t *testing.T) {
		if _, ok := Lip(nil).(lipgloss.NoColor); !ok {
			t.Errorf("Lip(nil) = %T, want lipgloss.NoColor", Lip(nil))
		}
	})

	t.Run("rgb becomes hex color", func(t *testing.T) {
		got, ok := Lip(hexColor("#7aa2f7")).(lipgloss.Color)
		if !ok {
			t.Fatalf("Lip(...) = %T, want lipgloss.Color", Lip(hexColor("#7aa2f7")))
		}
		if string(got) != "#7aa2f7" {
			t.Errorf("Lip(...) = %q, want #7aa2f7", string(got))
		}
	})

	t.Run("ansi color passes through", func(t *testing.T) {
		if got := Lip(lipgloss.Color("5")); string(got.(lipgloss.Color)) != "5" {
			t.Errorf("Lip(ansi) = %v, want 5", got)
		}
	})
}

// A theme declared in terminal color names cannot be blended. Returning the
// base made Selection and SearchMatch the background color itself, so
// selections and search hits were invisible.
func TestBlend_NamedColorsStayVisible(t *testing.T) {
	base, accent := parseThemeColor("black"), parseThemeColor("yellow")
	if base == nil || accent == nil {
		t.Fatal("setup: named colors did not parse")
	}
	if got := Blend(base, accent, 25); got != accent {
		t.Errorf("Blend(named, named) = %v, want the accent %v", got, accent)
	}
	if got := Blend(hexColor("#000000"), accent, 25); got != accent {
		t.Errorf("Blend(hex, named) = %v, want the accent %v", got, accent)
	}
}
