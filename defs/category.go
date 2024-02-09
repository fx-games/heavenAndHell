package defs

import "strings"

type ItemCategory int

func (c ItemCategory) HasAlwaysOneCharge() bool {
	switch c {
	case ItemCategoryPotion:
		return true
	case ItemCategoryScroll:
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
	case ItemCategoryAccessory:
		fallthrough
	case ItemCategoryThrowable:
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
	case ItemCategoryAccessory:
		return "Accessory"
	case ItemCategoryThrowable:
		return "Throwable"
	case ItemCategoryOther:
		return "Other"
	}
	return "Invalid"
}

const ( // defines the sort order of the inventory
	ItemCategoryInvalid ItemCategory = iota - 1
	ItemCategoryPotion
	ItemCategoryScroll
	ItemCategoryWand
	ItemCategoryLightSource
	ItemCategoryWeapon
	ItemCategoryArmor
	ItemCategoryAccessory
	ItemCategoryThrowable
	ItemCategoryOther
)

func ItemCategoryFromString(str string) ItemCategory {
	str = strings.ToLower(str)
	switch str {
	case "potion":
		return ItemCategoryPotion
	case "scroll":
		return ItemCategoryScroll
	case "wand":
		return ItemCategoryWand
	case "light":
		return ItemCategoryLightSource
	case "weapon":
		return ItemCategoryWeapon
	case "armor":
		return ItemCategoryArmor
	case "accessory":
		return ItemCategoryAccessory
	case "throwable":
		return ItemCategoryThrowable
	}

	return -1
}
