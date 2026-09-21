package d1

import (
	"io/fs"
	"os"
	"testing"

	"rpg/files"
)

// testFS is the game's own overlay, rooted at the repo's files/ because tests run in the package directory.
// Going through SelectMod means the tests exercise the real resolution order instead of reaching into mods/d1.
func testFS() fs.FS {
	files.SetFileSystem(os.DirFS("../files").(files.FileSystem))
	files.SelectMod("d1")
	return files.GetFileSystem()
}

// TestModShadowsEngineAssets proves the overlay is really in the load path: the mod's copy of a data file wins
// over the engine's own assets/data. Without SelectMod this read would miss (or find the engine's fallback).
func TestModShadowsEngineAssets(t *testing.T) {
	overlaid, err := fs.ReadFile(testFS(), "assets/data/monsters.rec")
	if err != nil {
		t.Fatal(err)
	}
	fromMod, err := fs.ReadFile(os.DirFS("../files"), "mods/d1/data/monsters.rec")
	if err != nil {
		t.Fatal(err)
	}
	if string(overlaid) != string(fromMod) {
		t.Fatal("assets/data/monsters.rec did not resolve to the mod's copy")
	}
}
