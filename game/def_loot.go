package game

import (
	"math/rand"
	"rpg/common"
)

/*
*
Theory:
Items, effects, etc. are each assigned a quality level (qlvl).

When loot it created, we generate an item level (ilvl).

Monster dies -> ilvl = monster level (+4 if we drop a unique from a unique monster)
Chest opened -> ilvl = dungeon level * 2

drop stuff where qlvl <= ilvl.

choose enchantments where 1/2 * ilvl <= qlvl <= ilvl

For items on the ground,
*/
func (g *GameInstance) generateLoot(dungeonLevel int) *GameItem {
	source := g.randomSource
	diceRoll := source.Intn(100)
	var addedItem *GameItem
	switch {
	case diceRoll >= 0 && diceRoll < 10:
		addedItem = g.NewRandomWeapon(dungeonLevel)
	case diceRoll >= 10 && diceRoll < 20:
		addedItem = g.NewRandomArmorPiece(dungeonLevel)
	case diceRoll >= 20 && diceRoll < 30:
		addedItem = g.NewRandomAccessory(dungeonLevel)
	case diceRoll >= 30 && diceRoll < 40:
		addedItem = g.NewRandomLightSource(dungeonLevel)
	case diceRoll >= 40 && diceRoll < 60:
		addedItem = g.NewRandomWand(dungeonLevel)
	case diceRoll >= 60 && diceRoll < 80:
		addedItem = g.NewRandomPotion(dungeonLevel)
	case diceRoll >= 80 && diceRoll < 100:
		addedItem = g.NewRandomScroll(dungeonLevel)
	}
	if addedItem != nil && addedItem.IsEquippable() {
		rarityRoll := source.Intn(100)
		rarity := common.ItemRarityCommon
		if rarityRoll >= 80 && rarityRoll < 95 {
			rarity = common.ItemRarityMagic
		} else if rarityRoll >= 95 {
			rarity = common.ItemRarityUnique
		}

		g.setRarityAndEnchantments(source, addedItem, rarity, dungeonLevel)
	}
	return addedItem
}
func (g *GameInstance) setRarityAndEnchantments(source *rand.Rand, item *GameItem, rarity common.ItemRarity, itemLevel int) {
	enchantmentCount := 0
	if rarity == common.ItemRarityCommon {
		return
	}
	// TODO: this should actually change the icon of the item
	if rarity == common.ItemRarityMagic {
		enchantmentCount = source.Intn(1) + 1
	}
	if rarity == common.ItemRarityUnique {
		enchantmentCount = source.Intn(5) + 1
	}
	modifierCount, hookCount := randomSplit(source, enchantmentCount)

	if modifierCount > 0 {
		mods := g.NewModifiersForItem(source, itemLevel, modifierCount)
		item.SetEquippedModifiers(mods)
	}

	if hookCount > 0 {
		itemHooks := NewHooksForItem(source, itemLevel, hookCount)
		item.SetItemHooks(itemHooks)
	}
}

func randomSplit(source *rand.Rand, value int) (int, int) {
	firstPart := source.Intn(value + 1)
	restPart := value - firstPart
	return firstPart, restPart
}
