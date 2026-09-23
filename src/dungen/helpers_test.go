package dungen

import (
	"io/fs"
	"os"
	"testing"

	"rpg/files"
)

// testFS is the game's own overlay, rooted at the repo's files/ because tests run in the package directory.
func testFS() fs.FS {
	files.SetFileSystem(os.DirFS("../../files").(files.FileSystem))
	files.SelectMod("d1")
	return files.GetFileSystem()
}

var sharedLevels *Levels

// testLevels is the D1 mod's levels, loaded once for every test.
func testLevels(t testing.TB) *Levels {
	t.Helper()
	if sharedLevels == nil {
		l, err := LoadLevels(testFS())
		if err != nil {
			t.Fatal(err)
		}
		sharedLevels = l
	}
	return sharedLevels
}
