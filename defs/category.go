package defs

type ItemCategory int

func (c ItemCategory) HasAlwaysOneCharge() bool {
	switch c {
	case ItemCategoryPotion:
		return true
	case ItemCategoryScroll:
		return true
	case ItemCategoryFood:
		return true
	}
	return false
}

func (c ItemCategory) IsValid() bool {
	if c == ItemCategoryInvalid {
		return false
	}
	switch c {
	case ItemCategoryPotion:
		fallthrough
	case ItemCategoryScroll:
		fallthrough
	case ItemCategoryWand:
		fallthrough
	case ItemCategoryLightSource:
		fallthrough
	case ItemCategoryWeapon:
		fallthrough
	case ItemCategoryArmor:
		fallthrough
	case ItemCategoryRing:
		fallthrough
	case ItemCategoryAmulet:
		fallthrough
	case ItemCategoryThrowable:
		fallthrough
	case ItemCategoryFood:
		fallthrough
	case ItemCategoryAmmunition:
		fallthrough
	case ItemCategoryOther:
		return true
	default:
		return false
	}
}

func (c ItemCategory) ToString() string {
	switch c {
	case ItemCategoryPotion:
		return "Potion"
	case ItemCategoryScroll:
		return "Scroll"
	case ItemCategoryWand:
		return "Wand"
	case ItemCategoryLightSource:
		return "Light Source"
	case ItemCategoryWeapon:
		return "Weapon"
	case ItemCategoryArmor:
		return "Armor"
	case ItemCategoryRing:
		return "Ring"
	case ItemCategoryAmulet:
		return "Amulet"
	case ItemCategoryThrowable:
		return "Throwable"
	case ItemCategoryAmmunition:
		return "Ammunition"
	case ItemCategoryFood:
		return "Food"
	case ItemCategoryOther:
		return "Other"
	}
	return "Invalid"
}

const ( // defines the sort order of the inventory
	ItemCategoryInvalid ItemCategory = iota - 1
	ItemCategoryPotion
	ItemCategoryScroll
	ItemCategoryFood
	ItemCategoryWand
	ItemCategoryLightSource
	ItemCategoryWeapon
	ItemCategoryArmor
	ItemCategoryRing
	ItemCategoryAmulet
	ItemCategoryThrowable
	ItemCategoryAmmunition
	ItemCategoryOther
)
