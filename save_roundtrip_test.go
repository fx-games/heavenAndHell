package main

import (
	"os"
	"path/filepath"
	"rpg/common"
	"rpg/defs"
	"rpg/game"
	"rpg/geometry"
	"rpg/gridmap"
	"testing"
)

// exploredCells counts the cells the current map remembers having been seen.
func exploredCells(state *common.State) int {
	m := state.GetMap()
	n := 0
	for y := 0; y < m.MapSize().Y; y++ {
		for x := 0; x < m.MapSize().X; x++ {
			if m.IsExplored(geometry.Point{X: x, Y: y}) {
				n++
			}
		}
	}
	return n
}

// startInDungeon begins a game in the given slot and walks it down to dungeon level 1.
func startInDungeon(t *testing.T, slot int) *game.GameInstance {
	t.Helper()
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.SetAndInitUserInterface(&slotUI{DummyUI: NewDummyUI(), choose: slot})
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Aidan")
	state := theGame.GetWorldState()
	state.MapTransition(gridmap.Transition{TargetMap: defs.LevelNameFromDepth(1), TargetLocation: "ladder_up"})
	// walk the level a little, so "explored" is much more than what arriving at the stairs lights up
	m := state.GetMap()
	player := state.GetPlayer()
	for y := 0; y < m.MapSize().Y; y += 7 {
		for x := 0; x < m.MapSize().X; x += 7 {
			p := geometry.Point{X: x, Y: y}
			if !m.CanPlaceActorHere(p) {
				continue
			}
			m.MoveActor(player, p)
			player.SetDrawPosition(p.ToPointF())
			theGame.GetActionBroker().Execute(common.NewUpdateFoVAction(common.AbstractCause("test"), player))
		}
	}
	return theGame
}

// The autosave and the load that follows it must agree on the slot: LoadGame used to ask for the most recently
// written slot in the save directory rather than the one the running game autosaves into, which is slot 0 -
// "no such file or directory" - whenever the autosave wrote nothing.
func TestLoadGameReadsTheSlotTheGameSavedInto(t *testing.T) {
	theGame := startInDungeon(t, 2)
	theGame.SaveGame()
	if got := theGame.SaveSlot(); got != 2 {
		t.Fatalf("game autosaves into slot %d, want 2", got)
	}
	// another slot holding a save that is just as new: picking "the latest slot in the directory" reads this
	// one, because slot 1 comes first, and the running game silently changes hands
	saved, err := os.ReadFile(filepath.Join(game.SaveDir, "d1", "slot2.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(game.SaveDir, "d1", "slot1.json"), saved, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := theGame.LoadGame(); err != nil {
		t.Fatalf("load after autosave: %v", err)
	}
	if got := theGame.SaveSlot(); got != 2 {
		t.Fatalf("after the load the game autosaves into slot %d, want 2", got)
	}
}

// A load must not forget the map: the explored bitmap is per level and comes back whole, or a loaded game
// walks a dungeon it has already lit up in the dark.
func TestExploredCellsSurviveSaveLoad(t *testing.T) {
	theGame := startInDungeon(t, 1)
	before := exploredCells(theGame.GetWorldState())
	if before < 20 {
		t.Fatalf("only %d explored cells before the save; the test explores nothing", before)
	}
	theGame.SaveGame()
	if err := theGame.LoadGame(); err != nil {
		t.Fatal(err)
	}
	if after := exploredCells(theGame.GetWorldState()); after < before {
		t.Fatalf("load forgot explored cells (%d -> %d)", before, after)
	}
}
