package files

import (
	"testing"
	"testing/fstest"
)

func TestModShadowsAssets(t *testing.T) {
	base := fstest.MapFS{
		"assets/data/items.rec":  {Data: []byte("engine")},
		"assets/fonts/x.ttf":     {Data: []byte("font")},
		"mods/d1/data/items.rec": {Data: []byte("mod")},
	}
	sourceFileSystem, activeMod = base, ""
	SelectMod("d1")
	defer func() { activeMod = "" }()

	for _, c := range []struct{ path, want string }{
		{"assets/data/items.rec", "mod"}, // mod wins
		{"assets/fonts/x.ttf", "font"},   // engine fallback
	} {
		got, err := ReadFile(c.path)
		if err != nil || string(got) != c.want {
			t.Errorf("%s = %q, %v; want %q", c.path, got, err, c.want)
		}
	}
}
