package main

import (
	"os"
	"rpg/game"
	"testing"
)

// TestMain points the save slots at a scratch directory for the whole package: a test must never read or write
// the player's own saves, and a leftover slot from one test must not decide where the next one saves.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "heavenAndHell_tests")
	if err != nil {
		panic(err)
	}
	game.SaveDir = dir
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
