package main

import (
	"os"
	"path/filepath"
	"rpg/game"
	"rpg/util"
	"testing"
)

// recordingUI captures what the menus do instead of drawing them.
type recordingUI struct {
	*DummyUI
	quit     bool
	lastMenu []util.MenuItem
}

func (r *recordingUI) QuitGame()                                  { r.quit = true }
func (r *recordingUI) OpenUncancelableMenu(items []util.MenuItem) { r.lastMenu = items }

func itemNamed(items []util.MenuItem, name string) *util.MenuItem {
	for i := range items {
		if items[i].MainText == name {
			return &items[i]
		}
	}
	return nil
}

// The start menu offers a way out of the game, and the pause menu's way back to it saves first,
// so that Continue resumes where the player stopped rather than at the last level change.
func TestStartMenuQuitsAndBackToMainMenuSavesFirst(t *testing.T) {
	game.SaveDir = filepath.Join(t.TempDir(), "saves")
	theGame, _ := createGameAndUIContext()
	theGame.GetAudio().SetDisablePlayback(true)
	ui := &recordingUI{DummyUI: NewDummyUI()}
	theGame.SetAndInitUserInterface(ui)
	theGame.NewGame(theGame.GetGameData().ClassDefinitions[0], false, "")

	quit := itemNamed(theGame.StartMenuItems(false), "Quit")
	if quit == nil {
		t.Fatal("the start menu has no Quit option")
	}
	quit.Action()
	if !ui.quit {
		t.Fatal("Quit did not ask the game to quit")
	}

	// Leaving for the main menu must not throw away progress since the last save.
	if err := os.RemoveAll(game.SaveDir); err != nil {
		t.Fatal(err)
	}
	theGame.BackToMainMenu()
	if !game.HasSave() {
		t.Fatal("BackToMainMenu opened the menu without saving the running game")
	}
	if itemNamed(ui.lastMenu, "Continue") == nil {
		t.Fatalf("no Continue in the menu we returned to: %v", ui.lastMenu)
	}
}
