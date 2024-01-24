package common

type SlotName int

const (
	SlotNameNotEquippable SlotName = iota
	SlotNameLightSource
	SlotNameMainHand
	SlotNameOffHand
	SlotNameOneHandedWeapon
	SlotNameAmulet
	SlotNameRing
	SlotNameRingRight
	SlotNameRingLeft
	SlotNameTwoHandedWeapon
	SlotNameArmorTorso
	SlotNameArmorHead
	SlotNameQuickSlot1
	SlotNameQuickSlot2
	SlotNameQuickSlot3
	SlotNameQuickSlot4
	SlotNameQuickSlot5
)

type Equipment struct {
	slots map[SlotName]Item
}

func NewEquipment() *Equipment {
	return &Equipment{
		slots: make(map[SlotName]Item),
	}
}

func (e *Equipment) IsEquipped(item Item) bool {
	slotName := item.SlotName()
	if slotName == SlotNameTwoHandedWeapon {
		slotName = SlotNameMainHand
	}

	if slotName == SlotNameOneHandedWeapon {
		if slotItem, exists := e.slots[SlotNameMainHand]; exists && slotItem == item {
			return true
		}
		if slotItem, exists := e.slots[SlotNameOffHand]; exists && slotItem == item {
			return true
		}
	}
	if slotItem, exists := e.slots[slotName]; exists && slotItem == item {
		return true
	}
	return false
}

func (e *Equipment) Equip(item Item) {
	slotName := item.SlotName()
	if slotName == SlotNameTwoHandedWeapon {
		e.slots[SlotNameMainHand] = item
		delete(e.slots, SlotNameOffHand)
		return
	}

	if slotName == SlotNameOneHandedWeapon {
		if _, exists := e.slots[SlotNameMainHand]; !exists {
			e.slots[SlotNameMainHand] = item
			return
		}
		if _, exists := e.slots[SlotNameOffHand]; !exists {
			e.slots[SlotNameOffHand] = item
			return
		}
	}

	e.slots[slotName] = item
}

func (e *Equipment) UnEquip(item Item) {
	slotName := item.SlotName()
	if slotName == SlotNameTwoHandedWeapon {
		slotName = SlotNameMainHand
	}
	if slotName == SlotNameOneHandedWeapon {
		if slotItem, exists := e.slots[SlotNameMainHand]; exists && slotItem == item {
			e.UnEquipBySlot(SlotNameMainHand)
			return
		}
		if slotItem, exists := e.slots[SlotNameOffHand]; exists && slotItem == item {
			e.UnEquipBySlot(SlotNameOffHand)
			return
		}
	}

	e.UnEquipBySlot(slotName)
}

func (e *Equipment) UnEquipBySlot(hand SlotName) {
	delete(e.slots, hand)
}

func (e *Equipment) GetBySlot(hand SlotName) Item {
	return e.slots[hand]
}

func (e *Equipment) GetWeapons(attackType AttackType) []Item {
	isCorrectWeaponType := func(item Item) bool {
		if attackType == MeleeAttack {
			return item.IsMeleeWeapon()
		}
		if attackType == RangedAttack {
			return item.IsRangedWeapon()
		}
		return false
	}

	if twoHanded, exists := e.slots[SlotNameTwoHandedWeapon]; exists && isCorrectWeaponType(twoHanded) {
		return []Item{twoHanded}
	}
	var weapons []Item
	if offHand, exists := e.slots[SlotNameOffHand]; exists && isCorrectWeaponType(offHand) {
		weapons = append(weapons, offHand)
	}
	if mainHand, exists := e.slots[SlotNameMainHand]; exists && isCorrectWeaponType(mainHand) {
		weapons = append(weapons, mainHand)
	}
	return weapons
}

func (e *Equipment) GetArmor() []Item {
	var armor []Item
	for _, item := range e.slots {
		if item.IsArmor() {
			armor = append(armor, item)
		}
	}
	return armor

}

func (e *Equipment) GetItemsToReplace(item Item) []Item {
	var items []Item
	if item.SlotName() == SlotNameTwoHandedWeapon {
		items = appendIfNotNil(items, e.slots[SlotNameMainHand])
		items = appendIfNotNil(items, e.slots[SlotNameOffHand])
		return items
	}
	if item.SlotName() == SlotNameOneHandedWeapon {
		if e.isOneSlotAvailable(SlotNameMainHand, SlotNameOffHand) {
			return items
		} else {
			items = appendIfNotNil(items, e.slots[SlotNameMainHand])
			return items
		}
	}
	items = appendIfNotNil(items, e.slots[item.SlotName()])
	return items
}

func (e *Equipment) isOneSlotAvailable(slotOne SlotName, slotTwo SlotName) bool {
	itemAtSlot, slotOneExists := e.slots[slotOne]
	if slotOneExists && itemAtSlot.SlotName() == SlotNameTwoHandedWeapon {
		return false
	}
	itemAtSlotTwo, slotTwoExists := e.slots[slotTwo]
	if slotTwoExists && itemAtSlotTwo.SlotName() == SlotNameTwoHandedWeapon {
		return false
	}
	return !slotOneExists || !slotTwoExists
}

func appendIfNotNil(items []Item, item Item) []Item {
	if item != nil {
		return append(items, item)
	}
	return items
}

func (e *Equipment) HasRangedWeaponEquipped() bool {
	slotMain := e.GetBySlot(SlotNameMainHand)
	slotOff := e.GetBySlot(SlotNameOffHand)
	return (slotMain != nil && slotMain.IsRangedWeapon()) || (slotOff != nil && slotOff.IsRangedWeapon())
}

func (e *Equipment) HasMeleeWeaponEquipped() bool {
	slotMain := e.GetBySlot(SlotNameMainHand)
	slotOff := e.GetBySlot(SlotNameOffHand)
	return (slotMain != nil && slotMain.IsMeleeWeapon()) || (slotOff != nil && slotOff.IsMeleeWeapon())
}

func (e *Equipment) HasLightSourceEquipped() bool {
	lightSlot := e.GetBySlot(SlotNameLightSource)
	return lightSlot != nil && lightSlot.IsLightSource()
}

func (e *Equipment) GetLightSource() Item {
	return e.GetBySlot(SlotNameLightSource)
}
