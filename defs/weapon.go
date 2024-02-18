package defs

import (
	"rpg/util"
	"strings"
)

type WeaponDefinition struct {
	WeaponType         WeaponType
	Damage             util.Dice
	Speed              float64
	MaxRange           int
	AccuracyAtMaxRange float64
	AccuracyAtMinRange float64
	ArmorPenetration   int
	ProjectileIcon     int32
}

func (d WeaponDefinition) IsValid() bool {
	return d.WeaponType != WeaponTypeNone
}

type WeaponType int

func (t WeaponType) ToString() string {
	switch t {
	case WeaponTypeSword:
		return "Sword"
	case WeaponTypeAxe:
		return "Axe"
	case WeaponTypeBow:
		return "Bow"
	case WeaponTypeCrossbow:
		return "Crossbow"
	case WeaponTypeMace:
		return "Mace"
	case WeaponTypeStaff:
		return "Staff"
	case WeaponTypeShield:
		return "Shield"
	case WeaponTypeScythe:
		return "Scythe"
	case WeaponTypeSpear:
		return "Spear"
	case WeaponTypeWhip:
		return "Whip"
	case WeaponTypeDagger:
		return "Dagger"
	case WeaponTypeSling:
		return "Sling"
	default:
		return "Unknown"
	}
}

func (t WeaponType) GetInventorySlot() SlotName {
	switch t {
	case WeaponTypeSword, WeaponTypeAxe, WeaponTypeMace, WeaponTypeWhip:
		return SlotNameOneHandedWeapon
	case WeaponTypeStaff, WeaponTypeScythe, WeaponTypeSpear:
		return SlotNameTwoHandedWeapon
	case WeaponTypeShield:
		return SlotNameOffHand
	case WeaponTypeBow:
		return SlotNameMissileLauncher
	default:
		return SlotNameNotEquippable
	}
}

/*
- Swords
- Clubs/Maces
- Axes
- Staves
- Bows
*/
const (
	WeaponTypeNone WeaponType = iota
	WeaponTypeSword
	WeaponTypeAxe
	WeaponTypeMace
	WeaponTypeStaff
	WeaponTypeSpear
	WeaponTypeDagger
	WeaponTypeBow
	WeaponTypeCrossbow
	WeaponTypeSling
	WeaponTypeShield
	WeaponTypeScythe
	WeaponTypeWhip
)

func WeaponTypeFromString(str string) WeaponType {
	str = strings.ToLower(str)
	switch str {
	case "sword":
		return WeaponTypeSword
	case "axe":
		return WeaponTypeAxe
	case "mace":
		return WeaponTypeMace
	case "staff":
		return WeaponTypeStaff
	case "bow":
		return WeaponTypeBow
	case "crossbow":
		return WeaponTypeCrossbow
	case "shield":
		return WeaponTypeShield
	case "scythe":
		return WeaponTypeScythe
	case "spear":
		return WeaponTypeSpear
	case "whip":
		return WeaponTypeWhip
	case "dagger":
		return WeaponTypeDagger
	case "sling":
		return WeaponTypeSling
	default:
		return WeaponTypeNone
	}
}
