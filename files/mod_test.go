package files

import (
	"testing"
	"testing/fstest"
)

// assets/ is the active mod: no engine fallback, so a file the mod lacks is missing.
func TestModOwnsAssets(t *testing.T) {
	base := fstest.MapFS{
		"assets/data/items.rec":  {Data: []byte("engine")},
		"assets/fonts/x.ttf":     {Data: []byte("font")},
		"mods/d1/data/items.rec": {Data: []byte("mod")},
	}
	sourceFileSystem, activeMod = base, ""
	SelectMod("d1")
	defer func() { activeMod = "" }()

	for _, c := range []struct{ path, want string }{
		{"assets/data/items.rec", "mod"},
	} {
		got, err := ReadFile(c.path)
		if err != nil || string(got) != c.want {
			t.Errorf("%s = %q, %v; want %q", c.path, got, err, c.want)
		}
	}
	if _, err := ReadFile("assets/fonts/x.ttf"); err == nil {
		t.Error("assets/fonts/x.ttf: read from outside the mod")
	}
}
