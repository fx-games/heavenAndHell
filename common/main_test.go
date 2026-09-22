package common

import (
	"fmt"
	"os"
	"testing"

	"rpg/d1"
	"rpg/defs"
	"rpg/files"
)

// D1's tables are loaded from the mod, not compiled in, so the tests that walk them (the sound and speech
// coverage checks) need Load to have run first -- otherwise they range over empty maps and pass without
// checking anything.
// testMod is the game this package's tests load: D1's tables are what they walk.
const testMod = "d1"

func TestMain(m *testing.M) {
	files.SelectMod(testMod)
	if err := defs.LoadIcons(); err != nil {
		panic(err)
	}
	if _, err := d1.Load(files.GetFileSystem()); err != nil {
		fmt.Println("d1.Load:", err)
		os.Exit(1)
	}
	os.Exit(m.Run())
}
