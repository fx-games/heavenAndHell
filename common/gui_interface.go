package common

import (
	"rpg/audio"
	"rpg/defs"
	"rpg/util"
)

type UIContext struct {
	GameState    func() *State
	PlayerAction func() PlayerActions
	AudioPlayer  *audio.Player
	Animator     func() Animator
	DebugActions func() DebugActions
	GetLog       func() []string
	StartInfos   StartInfo
	Definitions  defs.DataContainer
	// StartMenuItems lists Continue/Load and New game; afterDeath ends autosaving (decision 11).
	StartMenuItems func(afterDeath bool) []util.MenuItem
	// BackToMainMenu saves the running game and reopens the start menu.
	BackToMainMenu func()
	// OpenSaveMenu and OpenLoadMenu put the running game into a save slot, or load one; back reopens the menu
	// they were reached from. Both are hidden in permadeath games (IsPermadeath).
	OpenSaveMenu func(back func())
	OpenLoadMenu func(back func())
	IsPermadeath func() bool
	QuitSaves    func() bool // quitting now also saves: the pause menu says "Save & Quit"
	// IsHellfire and EnableHellfire: a D1 save can switch to Hellfire content once, mid-game, never back.
	IsHellfire     func() bool
	EnableHellfire func()
}

func SortStacksForTextUITyped(stacks []InventoryStack, getLetterForStack func(stack InventoryStack) rune, selected func(invItemKey rune)) []*TextualStack {
	items := make([]*TextualStack, 0)
	SortInventory(stacks)
	for _, itemStack := range stacks {
		items = append(items, NewTextualStack(itemStack, getLetterForStack(itemStack), selected))
	}
	return items
}
