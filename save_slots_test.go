package main

import (
	"os"
	"path/filepath"
	"rpg/common"
	"rpg/game"
	"testing"
)

// slotUI answers the save slot screen with a fixed choice and remembers whether it was asked to confirm an
// overwrite, which is the one promise the slot system makes: no save is lost without a question.
type slotUI struct {
	*DummyUI
	choose    int
	confirmed string
	refuse    bool // answer "no" to the overwrite question
	asked     int  // how often the slot screen was opened
}

func (s *slotUI) OpenSaveSlots(title string, slots []common.SaveSlot, pick func(slot int), back func()) {
	s.asked++
	pick(s.choose)
}

func (s *slotUI) Confirm(message string, onConfirm func()) {
	s.confirmed = message
	if !s.refuse {
		onConfirm()
	}
}

// A new game asks for its slot even with free ones left, and writes the metadata the load screen shows; saving into another slot
// leaves the first one alone; and writing over a slot that holds somebody else's hero asks first, and writes
// nothing when the answer is no.
func TestSaveSlots(t *testing.T) {
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	ui := &slotUI{DummyUI: NewDummyUI(), choose: 1}
	theGame.SetAndInitUserInterface(ui)

	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Aidan")
	if ui.asked != 1 {
		t.Fatalf("a new game with every slot free picked one itself instead of asking (asked %d times)", ui.asked)
	}
	slots := game.ListSaveSlots()
	if len(slots) != game.MaxSaveSlots {
		t.Fatalf("got %d slots, want %d", len(slots), game.MaxSaveSlots)
	}
	if !slots[0].Used || slots[0].Name != "Aidan" || slots[0].Level != 1 || slots[0].SavedAt.IsZero() {
		t.Fatalf("slot 1 after a new game: %+v", slots[0])
	}
	if slots[1].Used {
		t.Fatalf("a new game filled more than one slot: %+v", slots[1])
	}

	// saving into an empty slot needs no question and leaves the slot it came from intact
	ui.choose = 3
	theGame.OpenSaveMenu(func() {})
	if ui.confirmed != "" {
		t.Fatalf("saving into an empty slot asked to confirm: %q", ui.confirmed)
	}
	slots = game.ListSaveSlots()
	if !slots[0].Used || !slots[2].Used {
		t.Fatalf("after saving to slot 3: slot1=%v slot3=%v", slots[0].Used, slots[2].Used)
	}

	// slot 1 now holds a hero this game no longer autosaves into: writing over it must be confirmed
	ui.refuse = true
	ui.choose = 1
	theGame.OpenSaveMenu(func() {})
	if ui.confirmed == "" {
		t.Fatal("overwriting another save did not ask for confirmation")
	}
	if got := game.ListSaveSlots()[0]; !got.SavedAt.Equal(slots[0].SavedAt) || got.Name != "Aidan" {
		t.Fatal("slot 1 was written although the overwrite was refused")
	}

	// with every slot taken, a new hero has to ask too - even for the slot the running game came from
	ui.refuse = false
	for _, slot := range []int{2, 4, 5} {
		ui.choose = slot
		theGame.OpenSaveMenu(func() {})
	}
	if free := game.FirstFreeSaveSlot(); free != 0 {
		t.Fatalf("slot %d is still free after filling all of them", free)
	}
	ui.choose, ui.confirmed, ui.refuse = 5, "", true // slot 5 is the one this game autosaves into
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Warriv")
	if ui.confirmed == "" {
		t.Fatal("a new game replaced an existing save without asking")
	}
	if got := game.ListSaveSlots()[4]; got.Name == "Warriv" {
		t.Fatal("the new game was started although the replacement was refused")
	}
}

// A savegame.json from before slots existed shows up as slot 1, so an in-progress game survives the upgrade.
func TestLegacySaveMigratesToSlotOne(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	game.SaveDir = filepath.Join(dir, "saves")
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	theGame.SetAndInitUserInterface(&slotUI{DummyUI: NewDummyUI(), choose: 1})
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "Moreina")

	// put the save back where the old single-slot game kept it: in the working directory, next to "saves"
	saved, err := os.ReadFile(filepath.Join(game.SaveDir, "slot1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(game.SaveDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd)
	defer func() { game.SaveDir = filepath.Join(dir, "saves") }() // never leave the real save directory armed
	game.SaveDir = "saves"                                        // only the real save directory migrates, so the test uses it inside its own temp cwd
	if err := os.WriteFile(game.LegacySaveFile, saved, 0o644); err != nil {
		t.Fatal(err)
	}
	if slot := game.ListSaveSlots()[0]; !slot.Used || slot.Name != "Moreina" {
		t.Fatalf("the old save did not turn up in slot 1: %+v", slot)
	}
	if _, err := os.Stat(game.LegacySaveFile); err == nil {
		t.Fatal("the old save file is still there after the move")
	}
}
