package common

import (
	"image/color"
	"rpg/util"
)

type Inventory struct {
	items         []Item
	maxItemStacks int
}

func NewInventory(maxItemStacks int) *Inventory {
	return &Inventory{
		items:         make([]Item, 0),
		maxItemStacks: maxItemStacks,
	}
}

func (i *Inventory) Items() []Item {
	return i.items
}
type InventoryStack []Item

func (i InventoryStack) Name() string {
	return i[0].Name()
}

func (i InventoryStack) GetTooltipLines() []util.LabelText {
	return i[0].GetTooltipLines()
}

func (i InventoryStack) InventoryIcon() int32 {
	return i[0].InventoryIcon()
}

func (i InventoryStack) TintColor() color.Color {
	return i[0].TintColor()
}

func (i InventoryStack) Counter() int {
	return len(i)
}

func (i *Inventory) StackedItems() []InventoryStack {
	stacks := make([]InventoryStack, 0)
	for _, item := range i.items {
		found := false
		for stackIndex, stack := range stacks {
			if stack[0].CanStackWith(item) {
				stack = append(stack, item)
				stacks[stackIndex] = stack
				found = true
				break
			}
		}
		if !found {
			stacks = append(stacks, []Item{item})
		}
	}
	return stacks
}

func (i *Inventory) Remove(item Item) {
	for idx, invItem := range i.items {
		if invItem == item {
			i.items = append(i.items[:idx], i.items[idx+1:]...)
			return
		}
	}
}

func (i *Inventory) Has(item Item) bool {
	for _, invItem := range i.items {
		if invItem == item {
			return true
		}
	}
	return false
}

func (i *Inventory) Add(item Item) {
	i.items = append(i.items, item)
}

func (i *Inventory) IsEmpty() bool {
	return len(i.items) == 0
}

func (i *Inventory) IsFull() bool {
	return len(i.StackedItems()) == i.maxItemStacks
}
