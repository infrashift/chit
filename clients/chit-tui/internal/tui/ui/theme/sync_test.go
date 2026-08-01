package theme

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/infrashift/chit/clients/chit-tui/internal/config"
)

// The palette slot list is duplicated across four places: the Theme struct,
// config.ThemeColorKeys, the #Theme CUE definition, and the key lookups in
// LoadLocal. Nothing in the compiler ties them together, so a slot added to
// one and forgotten in another silently stops being loadable or validated.
// These tests are the only thing holding that correspondence.

// themeColorFieldCount counts the color-typed fields on Theme.
func themeColorFieldCount(t *testing.T) int {
	t.Helper()

	colorType := reflect.TypeOf((*color.Color)(nil)).Elem()
	typ := reflect.TypeOf(Theme{})

	n := 0
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).Type == colorType {
			n++
		}
	}
	return n
}

func TestThemeStructMatchesKeyLists(t *testing.T) {
	got := themeColorFieldCount(t)
	want := len(config.ThemeColorKeys) + len(config.ThemeDerivedKeys)

	if got != want {
		t.Errorf("Theme has %d color fields but the key lists name %d "+
			"(%d palette + %d derived); a slot was added without updating "+
			"config.ThemeColorKeys, schema/theme.cue, and LoadLocal",
			got, want, len(config.ThemeColorKeys), len(config.ThemeDerivedKeys))
	}
}

// Every key the schema accepts must survive the round trip into a Theme. This
// catches a key that exists in the list and the schema but was never wired
// into LoadLocal's lookups — the failure mode that produces a theme silently
// missing one color.
func TestEveryPaletteKeyReachesTheTheme(t *testing.T) {
	// A distinct color per key, so a slot wired to the wrong key shows up as
	// a mismatch rather than coincidentally matching.
	body := "name = \"Sync\"\n"
	want := make(map[string]string, len(config.ThemeColorKeys))

	for i, key := range config.ThemeColorKeys {
		hex := distinctHex(i)
		want[key] = hex
		body += key + " = \"" + hex + "\"\n"
	}

	dir := writeTheme(t, "sync", body)
	th, _, err := LoadLocal("sync", dir)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}

	got := map[string]string{
		"background":      colorToHex(th.Background),
		"foreground":      colorToHex(th.Foreground),
		"subtle":          colorToHex(th.Subtle),
		"accent":          colorToHex(th.Accent),
		"error":           colorToHex(th.Error),
		"success":         colorToHex(th.Success),
		"warning":         colorToHex(th.Warning),
		"border":          colorToHex(th.Border),
		"active_border":   colorToHex(th.ActiveBorder),
		"highlight":       colorToHex(th.Highlight),
		"muted":           colorToHex(th.Muted),
		"username":        colorToHex(th.Username),
		"timestamp":       colorToHex(th.Timestamp),
		"unread_badge":    colorToHex(th.UnreadBadge),
		"pin_badge":       colorToHex(th.PinBadge),
		"channel_active":  colorToHex(th.ChannelActive),
		"mention_badge":   colorToHex(th.MentionBadge),
		"mention_text":    colorToHex(th.MentionText),
		"mention_self_bg": colorToHex(th.MentionSelfBg),
		"tag_badge":       colorToHex(th.TagBadge),
	}

	// The assertion map itself must cover the key list, or a new key could be
	// missed here too.
	if len(got) != len(config.ThemeColorKeys) {
		t.Fatalf("this test checks %d keys but the list names %d; update the map",
			len(got), len(config.ThemeColorKeys))
	}

	for key, wantHex := range want {
		gotHex, ok := got[key]
		if !ok {
			t.Errorf("key %q is in ThemeColorKeys but this test does not check it", key)
			continue
		}
		if gotHex != wantHex {
			t.Errorf("key %q loaded as %s, want %s — it is probably wired to "+
				"the wrong Theme field in LoadLocal", key, gotHex, wantHex)
		}
	}
}

// distinctHex produces a different color per index so misrouted keys are
// visible rather than accidentally equal.
func distinctHex(i int) string {
	const digits = "0123456789abcdef"
	hi := digits[(i/16)%16]
	lo := digits[i%16]
	return "#" + string(hi) + string(lo) + string(hi) + string(lo) + string(hi) + string(lo)
}

func TestDerivedKeysReachTheTheme(t *testing.T) {
	body := completeThemeBody() +
		"selection = \"#111111\"\nsearch_match = \"#222222\"\nsearch_match_active = \"#333333\"\n"

	dir := writeTheme(t, "derived", body)
	th, _, err := LoadLocal("derived", dir)
	if err != nil {
		t.Fatalf("LoadLocal: %v", err)
	}

	tests := []struct{ key, got, want string }{
		{"selection", colorToHex(th.Selection), "#111111"},
		{"search_match", colorToHex(th.SearchMatch), "#222222"},
		{"search_match_active", colorToHex(th.SearchMatchActive), "#333333"},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %s, want %s", tc.key, tc.got, tc.want)
		}
	}
}

// completeThemeBody builds a valid theme body from the canonical key list.
func completeThemeBody() string {
	body := "name = \"Generated\"\n"
	for _, key := range config.ThemeColorKeys {
		body += key + " = \"#7aa2f7\"\n"
	}
	return body
}
