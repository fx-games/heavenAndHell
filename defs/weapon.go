package defs

import (
	"rpg/util"
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
	IsTwoHanded        bool
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
	WeaponTypeSpear
	WeaponTypeDagger
	WeaponTypeBow
	WeaponTypeCrossbow
	WeaponTypeSling
	WeaponTypeShield
	WeaponTypeScythe
	WeaponTypeWhip
)
