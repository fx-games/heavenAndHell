package common

import (
	"fmt"
	"image/color"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
	"strings"
)

type ThrowStats struct {
	Damage        stats.Interval
	Requirements  map[stats.Name]int
	OnHitEffect   HitEffect
	ChanceToBreak float64
	HitZone       AoEResolver
}

func (s ThrowStats) Clone() *ThrowStats {
	return &ThrowStats{
		Damage:        s.Damage,
		Requirements:  s.Requirements,
		OnHitEffect:   s.OnHitEffect,
		ChanceToBreak: s.ChanceToBreak,
		HitZone:       s.HitZone,
	}
}
type WeaponStats struct {
	Damage                      stats.Interval
	Requirements                map[stats.Name]int
	Durability                  *stats.Resource
	Type                        WeaponType
	AttackSpeedFactor           float64
	MeleeRange                  int
	Plus 						int	// Plus is enchantment that gets added to damage
	DamagePerLocationPrediction func(attackerPos, targetPos geometry.Point, damage stats.Interval, weaponCount int) []LocationDamagePrediction
}

func (s WeaponStats) ToString(forActor *Actor) []util.LabelText {
	baseInfo := []util.LabelText{
		LabelTextFromString(fmt.Sprintf("Type: %s", s.Type.ToString())),
		LabelTextFromString(fmt.Sprintf("Damage: %s", s.Damage.ToString())),
		LabelTextFromString(fmt.Sprintf("Enchantment: +%d", s.Plus)),
		LabelTextFromString(fmt.Sprintf("Durability: %s", s.Durability.ToString())),
		LabelTextFromString(fmt.Sprintf("Attack Speed: %.2f", s.AttackSpeedFactor)),
	}

	var requirementsInfo []util.LabelText
	if len(s.Requirements) > 0 {
		for attr, value := range s.Requirements {
			var reqColor color.Color
			reqColor = DarkTextColor
			if forActor.GetStats().GetAttribute(attr) < value {
				reqColor = NegativeHighlightColor
			}
			requirementsInfo = append(requirementsInfo, util.LabelText{
				Text:      fmt.Sprintf("%s: %d", string(attr), value),
				TextColor: reqColor,
			})
		}
	}

	if len(requirementsInfo) > 0 {
		baseInfo = append(baseInfo, LabelTextFromString(""), LabelTextFromString("Requirements"))
		baseInfo = append(baseInfo, requirementsInfo...)
	}

	return baseInfo
}
func (s WeaponStats) ToTableString() []string {
	baseInfo := []util.TableRow{
		{Label: "Type:", Columns: []string{s.Type.ToString()}},
		{Label: "Damage:", Columns: []string{s.Damage.ToString()}},
		{Label: "Durability:", Columns: []string{s.Durability.ToString()}},
		{Label: "Attack Speed:", Columns: []string{fmt.Sprintf("%.2f", s.AttackSpeedFactor)}},
	}
	var weaponStats []string
	weaponStats = util.TableLayout(baseInfo)
	var requirementsInfo []util.TableRow
	if len(s.Requirements) > 0 {
		for attr, value := range s.Requirements {
			requirementsInfo = append(requirementsInfo, util.TableRow{
				Label:   fmt.Sprintf(" %s:", string(attr)),
				Columns: []string{fmt.Sprintf("%d", value)},
			})
		}
	}

	if len(requirementsInfo) > 0 {
		weaponStats = append(weaponStats, "", "Requirements")
		weaponStats = append(weaponStats, util.TableLayout(requirementsInfo)...)
	}

	return weaponStats
}
func (s WeaponStats) GetDamagePerLocationPrediction(attackerPos, targetPos geometry.Point, damage stats.Interval, weaponCount int) []LocationDamagePrediction {
	if s.DamagePerLocationPrediction == nil {
		return []LocationDamagePrediction{{Location: targetPos, Damage: damage.Div(weaponCount)}}
	}
	return s.DamagePerLocationPrediction(attackerPos, targetPos, damage, weaponCount)
}
func (s WeaponStats) IsMeleeWeapon() bool {
	return s.Type == WeaponTypeSword || s.Type == WeaponTypeAxe || s.Type == WeaponTypeMace || s.Type == WeaponTypeStaff || s.Type == WeaponTypeScythe
}

func (s WeaponStats) IsRangedWeapon() bool {
	return s.Type == WeaponTypeBow
}

func (s WeaponStats) NameParts() string {
	// angband style (3, +3)
	return fmt.Sprintf("(%d-%d, +%d)", s.Damage.Min(), s.Damage.Max(), s.Plus)
}

func (s WeaponStats) Clone() *WeaponStats {
	return &WeaponStats{
		Damage:                      s.Damage,
		Requirements:                s.Requirements,
		Durability:                  stats.NewResource(s.Durability.Min(), s.Durability.GetMax, s.Durability.Value()),
		Type:                        s.Type,
		AttackSpeedFactor:           s.AttackSpeedFactor,
		MeleeRange:                  s.MeleeRange,
		DamagePerLocationPrediction: s.DamagePerLocationPrediction,
	}
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
	case WeaponTypeCrossBow:
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
	WeaponTypeBow
	WeaponTypeCrossBow
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
		return WeaponTypeCrossBow
	case "shield":
		return WeaponTypeShield
	case "scythe":
		return WeaponTypeScythe
	case "spear":
		return WeaponTypeSpear
	case "whip":
		return WeaponTypeWhip
	default:
		return WeaponTypeNone
	}
}