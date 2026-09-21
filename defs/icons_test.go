package defs

import (
	"strings"
	"testing"
	"testing/fstest"

	"rpg/files"
)

// A mod that ships icon files but leaves a name out used to resolve it to 0 and render tile 0 forever. It must
// fail the load instead, naming every icon it did not supply -- all of them, not just the first.
func TestLoadIconsNamesEveryMissingIcon(t *testing.T) {
	const actors = "internal_name: class.warrior_male\nicon: 7\n"
	mod := fstest.MapFS{
		"mods/skimpy/data/icons/actors.rec": {Data: []byte(actors)},
		"mods/skimpy/data/icons/items.rec":  {},
		"mods/skimpy/data/icons/skills.rec": {},
		"mods/skimpy/data/icons/tiles.rec":  {},
	}
	real := files.GetFileSystem()
	files.SetFileSystem(mod)
	files.SelectMod("skimpy")
	defer files.SetFileSystem(real) // the real tree back, or every later test in this package reads the fake one

	err := LoadIcons()
	if err == nil {
		t.Fatal("LoadIcons succeeded for a mod that defines one icon out of dozens")
	}
	for _, want := range []string{"tile.grass", "class.rogue_male", "corpse.default", "effect.arrow"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name the missing icon %q: %v", want, err)
		}
	}
	if strings.Contains(err.Error(), "class.warrior_male") {
		t.Errorf("error names an icon the mod DID supply: %v", err)
	}
	if IconCharacterWarriorMale != 7 {
		t.Errorf("supplied icon not bound: got %d, want 7", IconCharacterWarriorMale)
	}
}
