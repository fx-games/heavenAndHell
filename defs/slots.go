package defs

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
	case SlotNameQuiver:
		return "Quivered"
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
