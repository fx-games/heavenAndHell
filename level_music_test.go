package main

import (
	"rpg/d1"
	"rpg/defs"
	"testing"
)

// music_start(leveltype): a set level plays its own type's theme - the Poisoned Water, off level 2, is a cave, and
// Lazarus' lair on 15 is built from (and sounds like) the cathedral.
func TestD1LevelTypeForMusic(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	for name, want := range map[string]int{"!gen_pwater_2": d1.DTypeCaves, "!gen_lazarus_15": d1.DTypeCathedral,
		defs.LevelNameFromDepth(1): d1.DTypeCathedral, defs.LevelNameFromDepth(6): d1.DTypeCatacombs, defs.LevelNameFromDepth(14): d1.DTypeHell} {
		if got := s.D1LevelType(name); got != want {
			t.Errorf("%s: level type %d, want %d", name, got, want)
		}
	}
}
