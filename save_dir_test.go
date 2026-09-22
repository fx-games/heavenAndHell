package main

import (
	"os"
	"rpg/defs"
	"rpg/files"
	"rpg/game"
	"testing"
)

// TestMain points the save slots at a scratch directory for the whole package: a test must never read or write
// the player's own saves, and a leftover slot from one test must not decide where the next one saves.
// testMod is the game the package's tests play unless one names its own: most were written against D1.
const testMod = "d1"

func TestMain(m *testing.M) {
	files.SelectMod(testMod) // content loads through the overlay here too, not just in the game
	modOverride = testMod    // and the user's config.rec (whichever mod they last picked) never decides
	if err := defs.LoadIcons(); err != nil {
		panic(err)
	}
	dir, err := os.MkdirTemp("", "heavenAndHell_tests")
	if err != nil {
		panic(err)
	}
	game.SaveDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
