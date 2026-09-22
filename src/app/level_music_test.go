package app

import "testing"

// music_start(leveltype), as data: a set level plays its own type's theme - the Poisoned Water, off level 2, is a
// cave, and Lazarus' lair on 15 is built from (and sounds like) the cathedral.
func TestD1LevelMusic(t *testing.T) {
	g, _ := createGameAndUIContext()
	g.GetAudio().SetDisablePlayback(true)
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	for id, want := range map[string]string{"town": "town", "dark_passage": "caves", "lazarus_lair": "cathedral",
		levelAt(s, 1): "cathedral", levelAt(s, 6): "catacombs", levelAt(s, 14): "hell"} {
		if got := s.Level(id).Music; got != want {
			t.Errorf("%s: music %q, want %q", id, got, want)
		}
	}
}

// A dungen level's music follows its tiles.
func TestHHLevelMusic(t *testing.T) {
	g := newModGame(t, "heavenandhell", NewDummyUI())
	g.NewGame(g.GetGameData().ClassDefinitions[0], false, "")
	s := g.GetWorldState()
	for dlvl, want := range map[int]string{1: "cathedral", 3: "catacombs", 5: "caves", 7: "hell", 11: "cathedral"} {
		if got := s.Level(levelAt(s, dlvl)).Music; got != want {
			t.Errorf("level %d: music %q, want %q", dlvl, got, want)
		}
	}
}
