package common

import (
	"rpg/audio"
	"rpg/defs"
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
}

func SortStacksForTextUITyped(stacks []InventoryStack, getLetterForStack func(stack InventoryStack) rune, selected func(invItemKey rune)) []*TextualStack {
	items := make([]*TextualStack, 0)
	SortInventory(stacks)
	for _, itemStack := range stacks {
		items = append(items, NewTextualStack(itemStack, getLetterForStack(itemStack), selected))
	}
	return items
}
