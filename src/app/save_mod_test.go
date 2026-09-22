package app

import (
	"os"
	"path/filepath"
	"rpg/files"
	"rpg/src/game"
	"strings"
	"testing"
)

// A save belongs to the mod it was played on: another mod neither lists it nor loads it, even when the file is
// put where that mod looks.
func TestSavesBelongToTheirMod(t *testing.T) {
	old := game.SaveDir
	t.Cleanup(func() { game.SaveDir = old })
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	d1Game := newModGame(t, "d1", NewDummyUI())
	d1Game.NewGame(d1Game.GetGameData().ClassDefinitions[0], false, "")
	d1Game.SaveToSlot(1)
	if !game.ListSaveSlots()[0].Used {
		t.Fatal("the D1 save is not listed under D1")
	}

	hh := newModGame(t, "heavenandhell", NewDummyUI())
	for _, slot := range game.ListSaveSlots() {
		if slot.Used {
			t.Errorf("H&H lists slot %d, a D1 game", slot.Slot)
		}
	}
	if err := hh.LoadSlot(1); err == nil {
		t.Error("H&H loaded D1's game")
	}
	// the file itself, dropped into H&H's slots, is still D1's
	data, err := os.ReadFile(filepath.Join(game.SaveDir, "d1", "slot1.json"))
	if err != nil {
		t.Fatal(err)
	}
	_ = os.MkdirAll(filepath.Join(game.SaveDir, "heavenandhell"), 0o755)
	_ = os.WriteFile(filepath.Join(game.SaveDir, "heavenandhell", "slot2.json"), data, 0o644)
	if err := hh.LoadSlot(2); err == nil || !strings.Contains(err.Error(), `"d1"`) {
		t.Errorf("a D1 save in H&H's slots: %v", err)
	}

	files.SelectMod("d1")
	if !game.ListSaveSlots()[0].Used {
		t.Error("back on D1, its save is gone")
	}
}
