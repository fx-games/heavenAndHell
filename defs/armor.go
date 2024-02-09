package defs

import (
	"rpg/stats"
	"strings"
)

type ArmorType int

func ArmorTypeFromString(str string) ArmorType {
	str = strings.ToLower(str)
	switch str {
	case "helmet":
		return ArmorTypeHelmet
	case "cuirass":
		return ArmorTypeCuirass
	case "gloves":
		return ArmorTypeGloves
	case "boots":
		return ArmorTypeBoots
	case "cape":
		return ArmorTypeCape
	case "amulet":
		return ArmorTypeAmulet
	case "ring":
		return ArmorTypeRing
	case "shield":
		return ArmorTypeShield
	case "defensive_weapon":
		return ArmorTypeDefensiveWeapon
	}
	return ArmorTypeNone
}

func (t ArmorType) ToString() string {
	switch t {
	case ArmorTypeHelmet:
		return "Helmet"
	case ArmorTypeCuirass:
		return "Cuirass"
	case ArmorTypeGloves:
		return "Gloves"
	case ArmorTypeBoots:
		return "Boots"
	case ArmorTypeCape:
		return "Cape"
	case ArmorTypeAmulet:
		return "Amulet"
	case ArmorTypeRing:
		return "Ring"
	case ArmorTypeShield:
		return "Shield"
	case ArmorTypeDefensiveWeapon:
		return "Defensive Weapon"
	}
	return "Unknown"
}

func (t ArmorType) GetInventorySlot() SlotName {
	switch t {
	case ArmorTypeHelmet:
		return SlotNameArmorHead
	case ArmorTypeCuirass:
		return SlotNameArmorTorso
	case ArmorTypeGloves:
		return SlotNameArmorHands
	case ArmorTypeBoots:
		return SlotNameArmorFeet
	case ArmorTypeCape:
		return SlotNameArmorBack
	case ArmorTypeAmulet:
		return SlotNameAmulet
	case ArmorTypeRing:
		return SlotNameRing
	case ArmorTypeShield:
		return SlotNameOffHand
	case ArmorTypeNone:
		return SlotNameNotEquippable
	case ArmorTypeDefensiveWeapon:
		return SlotNameMainHand
	}
	return SlotNameNotEquippable
}

const (
	ArmorTypeNone ArmorType = iota
	ArmorTypeHelmet
	ArmorTypeCuirass
	ArmorTypeBoots
	ArmorTypeGloves
	ArmorTypeCape
	ArmorTypeAmulet
	ArmorTypeRing
	ArmorTypeShield
	ArmorTypeDefensiveWeapon
)

type ArmorDefinition struct {
	ArmorType  ArmorType
	ArmorClass stats.Interval
}

func (d ArmorDefinition) IsValid() bool {
	return d.ArmorType != ArmorTypeNone
}
