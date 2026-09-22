package textui

import (
	"os"
	"testing"

	"github.com/memmaker/go/textiles"
)

// The theme's colors must all resolve, or the UI draws black on black. A palette entry in the wrong rec record
// (a blank line starts a new one) is easy to miss and impossible to see in a diff.
func TestAsciiThemeColorsAreVisible(t *testing.T) {
	for _, mod := range []string{"d1", "heavenandhell"} {
		t.Run(mod, func(t *testing.T) { checkThemeColors(t, "../../files/mods/"+mod+"/data/ascii/") })
	}
}

func checkThemeColors(t *testing.T, dir string) {
	paletteFile, err := os.Open(dir + "palette.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer paletteFile.Close()
	themeFile, err := os.Open(dir + "ui.rec")
	if err != nil {
		t.Fatal(err)
	}
	defer themeFile.Close()

	theme := NewUIThemeFromRecords(themeFile, textiles.ReadPaletteFileOrDefault(paletteFile))

	background := theme.GetUIColor(UIColorUIBackground)
	for _, uiColor := range []UIColor{UIColorUIForeground, UIColorTextForegroundHighlighted, UIColorBorderForeground,
		UIColorBorderForegroundFocus, UIColorStatusBarForeground} {
		foreground := theme.GetUIColor(uiColor)
		if foreground.A == 0 {
			t.Errorf("color %d is not in the palette", uiColor)
		}
		if foreground == background {
			t.Errorf("color %d is the background color: unreadable", uiColor)
		}
	}
}
