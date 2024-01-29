package common

import (
	"cmp"
	"fmt"
	"image/color"
	"rpg/util"
	"slices"
)

type Inventory struct {
	items         []Item
	maxItemStacks int
	letterToItem  map[rune]Item
	itemToLetter  map[Item]rune
}

func NewInventory(maxItemStacks int) *Inventory {
	return &Inventory{
		items:         make([]Item, 0),
		maxItemStacks: maxItemStacks,
		itemToLetter:  make(map[Item]rune),
		letterToItem:  make(map[rune]Item),
	}
}

func (i *Inventory) Items() []Item {
	return i.items
}
type InventoryStack []Item

func (i InventoryStack) Name() string {
	return appendStacks(i[0].Name(), len(i))
}

func appendStacks(name string, stackCount int) string {
	if stackCount > 1 {
		name = fmt.Sprintf("%s (x%d)", name, stackCount)
	}
	return name
}

func (i InventoryStack) PlusName() string {
	return appendStacks(i[0].PlusName(), len(i))
}

func (i InventoryStack) BandStyleName() string {
	return appendStacks(i[0].BandStyleName(), len(i))
}

func (i InventoryStack) String() string {
	return i.Name()
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
	return i.StackedItemsWithFilter(func(item Item) bool { return true })
}
func (i *Inventory) StackedItemsWithFilter(filter func(Item) bool) []InventoryStack {
	stacks := make([]InventoryStack, 0)
	for _, item := range i.items {
		if !filter(item) {
			continue
		}
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
			i.updateLetterMapping()
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
	i.updateLetterMapping()
}

func (i *Inventory) IsEmpty() bool {
	return len(i.items) == 0
}

func (i *Inventory) IsFull() bool {
	return len(i.StackedItems()) == i.maxItemStacks
}

func (i *Inventory) updateLetterMapping() {
	i.letterToItem = make(map[rune]Item)
	sortedItems := i.StackedItems()
	SortInventory(sortedItems)
	for idx, item := range sortedItems {
		firstItem := item[0]
		assignedLetter := rune('a' + idx)
		if idx >= 26 {
			assignedLetter = rune('A' + idx - 26)
		}
		i.letterToItem[assignedLetter] = firstItem
		i.itemToLetter[firstItem] = assignedLetter
	}
}

func (i *Inventory) GetItemByLetter(letter rune) Item {
	if i.letterToItem == nil {
		i.updateLetterMapping()
	}
	return i.letterToItem[letter]
}

func (i *Inventory) GetLetterForItemStack(item InventoryStack) rune {
	if i.itemToLetter == nil {
		i.updateLetterMapping()
	}
	firstItem := item[0]
	return i.itemToLetter[firstItem]
}

func (i *Inventory) AsColorCodedStrings() []string {
	items := i.StackedItems()
	SortInventory(items)
	var asStrings []string
	for _, item := range items {
		itemColor := ColorCodeFromColor(item.TintColor())
		itemLetter := i.GetLetterForItemStack(item)
		itemLine := fmt.Sprintf("[:white]%c) %s%s", itemLetter, itemColor, item.BandStyleName())
		if item.Counter() > 1 {
			itemLine = fmt.Sprintf("[:white]%c) %s%s (x%d)", itemLetter, itemColor, item.BandStyleName(), item.Counter())
		}
		asStrings = append(asStrings, itemLine)
	}
	return asStrings
}

func (i *Inventory) HasLightSource() bool {
	for _, item := range i.items {
		if item.IsLightSource() {
			return true
		}
	}
	return false
}

func SortInventory(stacks []InventoryStack) {
	slices.SortStableFunc(stacks, func(i, j InventoryStack) int {
		itemI := i[0]
		itemTypeOne := itemI.GetItemType()
		itemJ := j[0]
		itemTypeTwo := itemJ.GetItemType()
		if itemTypeOne != itemTypeTwo {
			return cmp.Compare(itemTypeOne, itemTypeTwo)
		} else {
			switch itemTypeOne {
			case ItemCategoryWeapon:
				weaponPartOne := itemI.GetWeaponStats()
				weaponPartTwo := itemJ.GetWeaponStats()
				return cmp.Compare(weaponPartOne.Type, weaponPartTwo.Type)
			case ItemCategoryArmor:
				armorPartOne := itemI.GetArmorStats()
				armorPartTwo := itemJ.GetArmorStats()
				return cmp.Compare(armorPartOne.Type, armorPartTwo.Type)
			case ItemCategoryWand:
				fallthrough
			case ItemCategoryPotion:
				fallthrough
			case ItemCategoryScroll:
				fallthrough
			default:
				return cmp.Compare(itemI.Name(), itemJ.Name())
			}
		}
	})
}