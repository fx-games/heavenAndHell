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
	ResetGame    func(class defs.LoadoutDefinition)
	Animator     func() Animator
	DebugActions func() DebugActions
	GetLog       func() []string
	StartInfos   StartInfo
	Definitions  defs.DataContainer
	// StartMenuItems lists Continue/Load and New game; afterDeath ends autosaving (decision 11).
	StartMenuItems func(afterDeath bool) []util.MenuItem
}

func SortStacksForTextUITyped(stacks []InventoryStack, getLetterForStack func(stack InventoryStack) rune, selected func(invItemKey rune)) []*TextualStack {
	items := make([]*TextualStack, 0)
	SortInventory(stacks)
	for _, itemStack := range stacks {
		items = append(items, NewTextualStack(itemStack, getLetterForStack(itemStack), selected))
	}
	return items
}
