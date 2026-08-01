package theme

import (
	"image/color"
	"testing"
)

func TestBuiltinsPopulateDerivedSlots(t *testing.T) {
	for name, ctor := range builtinThemes {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			slots := map[string]color.Color{
				"Selection":         th.Selection,
				"SearchMatch":       th.SearchMatch,
				"SearchMatchActive": th.SearchMatchActive,
			}
			for slot, c := range slots {
				if c == nil {
					t.Errorf("%s is nil; derive() should have computed it", slot)
				}
			}
		})
	}
}

// The active match must be visually distinct from an inactive one, otherwise
// "next match" navigation gives the user no feedback.
func TestSearchMatchActiveDiffersFromSearchMatch(t *testing.T) {
	for name, ctor := range builtinThemes {
		t.Run(name, func(t *testing.T) {
			th := ctor()
			if colorToHex(th.SearchMatch) == colorToHex(th.SearchMatchActive) {
				t.Errorf("SearchMatch and SearchMatchActive are both %s",
					colorToHex(th.SearchMatch))
			}
		})
	}
}

func TestDerivePreservesExplicitValues(t *testing.T) {
	explicit := hexColor("#ff0000")
	th := Theme{
		Background:  hexColor("#000000"),
		Accent:      hexColor("#00ff00"),
		Warning:     hexColor("#0000ff"),
		Selection:   explicit,
		SearchMatch: explicit,
	}.derive()

	if colorToHex(th.Selection) != "#ff0000" {
		t.Errorf("Selection = %s, want the explicit #ff0000", colorToHex(th.Selection))
	}
	if colorToHex(th.SearchMatch) != "#ff0000" {
		t.Errorf("SearchMatch = %s, want the explicit #ff0000", colorToHex(th.SearchMatch))
	}
	if th.SearchMatchActive == nil {
		t.Error("SearchMatchActive was unset and should have been derived")
	}
}

func TestDeriveIsIdempotent(t *testing.T) {
	once := TokyoNight()
	twice := once.derive()

	if colorToHex(once.Selection) != colorToHex(twice.Selection) {
		t.Errorf("Selection changed on second derive: %s then %s",
			colorToHex(once.Selection), colorToHex(twice.Selection))
	}
}

// A theme file that omits the derived keys should still get usable values,
// computed from whatever palette it did declare.
func TestParseJSONDerivesMissingSlots(t *testing.T) {
	th, err := ParseJSON([]byte(`{"name":"x","background":"#000000","accent":"#00ff00"}`))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if th.Selection == nil {
		t.Error("Selection is nil; it should have been derived from background+accent")
	}
}

func TestParseJSONHonoursExplicitDerivedSlot(t *testing.T) {
	th, err := ParseJSON([]byte(`{"name":"x","selection":"#123456"}`))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if got := colorToHex(th.Selection); got != "#123456" {
		t.Errorf("Selection = %s, want #123456", got)
	}
}
