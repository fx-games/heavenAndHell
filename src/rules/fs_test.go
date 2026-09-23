package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rpg/files"
)

// testFS is the game's own overlay, rooted at the repo's files/ because tests run in the package directory.
// Going through SelectMod means the tests exercise the real resolution order instead of reaching into mods/d1.
func testFS() fs.FS {
	files.SetFileSystem(os.DirFS("../../files").(files.FileSystem))
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
	fromMod, err := fs.ReadFile(os.DirFS("../../files"), "mods/d1/data/monsters.rec")
	if err != nil {
		t.Fatal(err)
	}
	if string(overlaid) != string(fromMod) {
		t.Fatal("assets/data/monsters.rec did not resolve to the mod's copy")
	}
}

// hellfireFS is testFS with Hellfire's classes, items and oils in place of the D1 mod's 1.09 tables. The D1 mod
// declares none of them; the engine still supports them, and these tests keep that support honest.
func hellfireFS() fs.FS { return overlayFS{testFS(), "testdata/hellfire"} }

type overlayFS struct {
	base fs.FS
	dir  string
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if rel, ok := strings.CutPrefix(name, "assets/data/"); ok {
		if f, err := os.Open(filepath.Join(o.dir, rel)); err == nil {
			return f, nil
		}
	}
	return o.base.Open(name)
}

// loadHellfire loads hellfireFS for one test and puts the D1 mod's tables back after it: the class, spell and oil
// tables are package state every other test reads.
func loadHellfire(t *testing.T) *Data {
	t.Helper()
	d, err := Load(hellfireFS())
	if err != nil {
		t.Fatal(err)
	}
	return d
}
