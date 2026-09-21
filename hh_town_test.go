package main

import (
	"rpg/common"
	"rpg/defs"
	"rpg/files"
	"rpg/game"
	"rpg/gridmap"
	"testing"
)

// A mod is done when you can play it. For Heaven & Hell that starts with every class starting a real game in its
// own town, and each of the town's three doors leading down and back to the same door - the path main's
// setupTownTransitions hardcoded in Go, and the mod now declares as data.
func TestHeavenAndHellTownIsPlayable(t *testing.T) {
	files.SelectMod("heavenandhell")
	_ = defs.LoadIcons() // a mod may still owe the engine an icon; what it DID supply is bound either way
	t.Cleanup(func() {
		files.SelectMod(common.DefaultMod)
		if err := defs.LoadIcons(); err != nil {
			t.Fatal(err)
		}
	})
	cfg := common.NewConfigurationFromRecFile(common.ConfigFile)
	g := game.NewGameInstance(common.StartInfo{ScreenSize: cfg.WindowSize, TileScale: 4, LDTKMapFile: "assets/maps.ldtk",
		SpriteSheet: cfg.SpriteSheet, FontInfos: common.FontInfo{TTFFile: cfg.DefaultFont, TTFSize: cfg.DefaultFontSize},
		UserConfiguration: &cfg, FileSystem: files.GetFileSystem()}, NewDummyUI())
	g.GetAudio().SetDisablePlayback(true)

	for _, class := range g.GetGameData().ClassDefinitions {
		g.NewGame(class, false, "")
		s := g.GetWorldState()
		if s.GetMap().GetName() != string(defs.LevelNameTown) {
			t.Fatalf("%s starts on %q, not in town", class.Name, s.GetMap().GetName())
		}
		for _, door := range []string{"to_cathedral", "to_catacombs", "to_caves"} {
			s.MapTransition(gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: "spawn"})
			down, ok := s.GetMap().GetTransitionAt(s.GetMap().GetNamedLocation(door))
			if !ok {
				t.Errorf("%s: the town's %s leads nowhere", class.Name, door)
				continue
			}
			s.MapTransition(down)
			d := s.GetMap()
			if d.GetName() != down.TargetMap {
				t.Errorf("%s: %s should lead to %s, got %s", class.Name, door, down.TargetMap, d.GetName())
				continue
			}
			up, ok := d.GetTransitionAt(d.GetNamedLocation(down.TargetLocation))
			if want := (gridmap.Transition{TargetMap: string(defs.LevelNameTown), TargetLocation: door}); !ok || up != want {
				t.Errorf("%s: the way back from %s leads to %+v, want %+v", class.Name, d.GetName(), up, want)
			}
		}
	}
}
