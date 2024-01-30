package common

type SlotName int

func (n SlotName) ToString() string {
	switch n {
	case SlotNameMainHand:
		return "Wielding"
	case SlotNameMissileLauncher:
		return "Shooting"
	case SlotNameOffHand:
		return "Off Hand"
	case SlotNameLightSource:
		return "Light source"
	case SlotNameOneHandedWeapon:
		return "One Handed Weapon"
	case SlotNameAmulet:
		return "Around neck"
	case SlotNameRing:
		return "Ring"
	case SlotNameRingRight:
		return "On right hand"
	case SlotNameRingLeft:
		return "On left hand"
	case SlotNameTwoHandedWeapon:
		return "Two Handed Weapon"
	case SlotNameArmorTorso:
		return "On body"
	case SlotNameArmorHead:
		return "On head"
	case SlotNameArmorFeet:
		return "On feet"
	case SlotNameArmorHands:
		return "On hands"
	case SlotNameArmorBack:
		return "On back"
	case SlotNameQuiver: return "Quivered"
	case SlotNameQuickSlot1:
		return "Quick Slot 1"
	case SlotNameQuickSlot2:
		return "Quick Slot 2"
	case SlotNameQuickSlot3:
		return "Quick Slot 3"
	case SlotNameQuickSlot4:
		return "Quick Slot 4"
	case SlotNameQuickSlot5:
		return "Quick Slot 5"
	}
	return "Unknown"
}


const (
	SlotNameNotEquippable SlotName = iota
	SlotNameMainHand
    SlotNameMissileLauncher
	SlotNameQuiver
	SlotNameOffHand
	SlotNameLightSource
	SlotNameOneHandedWeapon
	SlotNameAmulet
	SlotNameRing
	SlotNameRingRight
	SlotNameRingLeft
	SlotNameTwoHandedWeapon
	SlotNameArmorTorso
	SlotNameArmorHead
	SlotNameArmorFeet
	SlotNameArmorHands
	SlotNameArmorBack
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

	if slotName == SlotNameRing {
		if slotItem, exists := e.slots[SlotNameRingLeft]; exists && slotItem == item {
			return true
		}
		if slotItem, exists := e.slots[SlotNameRingRight]; exists && slotItem == item {
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

	if slotName == SlotNameRing {
		if _, exists := e.slots[SlotNameRingLeft]; !exists {
			e.slots[SlotNameRingLeft] = item
			return
		}
		if _, exists := e.slots[SlotNameRingRight]; !exists {
			e.slots[SlotNameRingRight] = item
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
	if slotName == SlotNameRing {
		if slotItem, exists := e.slots[SlotNameRingLeft]; exists && slotItem == item {
			e.UnEquipBySlot(SlotNameRingLeft)
			return
		}
		if slotItem, exists := e.slots[SlotNameRingRight]; exists && slotItem == item {
			e.UnEquipBySlot(SlotNameRingRight)
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
	if item.SlotName() == SlotNameRing {
		if e.isOneSlotAvailable(SlotNameRingLeft, SlotNameRingRight) {
			return items
		} else {
			items = appendIfNotNil(items, e.slots[SlotNameRingLeft])
			return items
		}
	}
	items = appendIfNotNil(items, e.slots[item.SlotName()])
	return items
}

func (e *Equipment) isOneSlotAvailable(slotOne SlotName, slotTwo SlotName) bool {
	_, slotOneExists := e.slots[slotOne]
	_, slotTwoExists := e.slots[slotTwo]
	return !slotOneExists || !slotTwoExists
}

func appendIfNotNil(items []Item, item Item) []Item {
	if item != nil {
		return append(items, item)
	}
	return items
}

func (e *Equipment) HasRangedWeaponEquipped() bool {
	slotMain := e.GetBySlot(SlotNameMissileLauncher)
	return slotMain != nil && slotMain.IsRangedWeapon()
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

func (e *Equipment) AllItems() []Item {
	var items []Item
	for _, item := range e.slots {
		items = append(items, item)
	}
	return items
}

func (e *Equipment) GetSlot(item Item) SlotName {
	for slot, slotItem := range e.slots {
		if slotItem == item {
			return slot
		}
	}
	return SlotNameNotEquippable
}

func (e *Equipment) HasThrowableQuivered() bool {
	slot := e.GetBySlot(SlotNameQuiver)
	return slot != nil && slot.IsThrowable()
}

func (e *Equipment) GetNextQuiveredThrowable() Item {
	slot := e.GetBySlot(SlotNameQuiver)
	if slot == nil {
		return nil
	}
	return slot
}

func (e *Equipment) HasWeaponEquipped() bool {
	slot := e.GetBySlot(SlotNameMainHand)
	return slot != nil && slot.IsWeapon()
}

func (e *Equipment) HasArmorEquipped() bool {
	slot := e.GetBySlot(SlotNameArmorTorso)
	return slot != nil && slot.IsArmor()
}
