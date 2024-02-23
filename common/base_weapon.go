package common

import (
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"image/color"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
	"strings"
)

type ThrowStats struct {
	Damage        stats.Interval
	OnHitEffect   *HitEffect
	ChanceToBreak float64
	HitZone       *AoEResolver
}

func (s ThrowStats) Clone() *ThrowStats {
	return &ThrowStats{
		Damage:        s.Damage,
		OnHitEffect:   s.OnHitEffect,
		ChanceToBreak: s.ChanceToBreak,
		HitZone:       s.HitZone,
	}
}

func (s ThrowStats) ToLabelText() []util.LabelText {
	var throwInfo []util.LabelText
	if s.Damage.NotZero() {
		throwInfo = append(throwInfo, util.LabelText{
			Text:      fmt.Sprintf("Damage when thrown: %s", s.Damage.ToString()),
			TextColor: defs.OffWhite,
		})
	}
	if s.ChanceToBreak > 0 {
		throwInfo = append(throwInfo, util.LabelText{
			Text:      fmt.Sprintf("Chance to break when thrown: %.0f%%", s.ChanceToBreak*100),
			TextColor: defs.OffWhite,
		})
	}
	return throwInfo
}

type WeaponStats struct {
	Damage             util.Dice
	Requirements       map[stats.Name]int
	Durability         *stats.Resource
	Type               defs.WeaponType
	AttackSpeedFactor  float64
	MeleeRange         int
	Plus               int // Plus is enchantment that gets added to damage
	MaxRange           int
	AccuracyAtMaxRange float64
	AccuracyAtMinRange float64
	ArmorPenetration   int
	ProjectileIcon int32
	IsTwoHanded    bool
}

func (s WeaponStats) ToString(forActor *Actor) []util.LabelText {
	baseInfo := []util.LabelText{
		util.LabelTextFromStringWithColor(fmt.Sprintf("Weapon Type: %s", s.Type.ToString()), defs.OffWhite),
		util.LabelTextFromStringWithColor(fmt.Sprintf("Damage: %s", s.Damage.String()), defs.OffWhite),
		util.LabelTextFromStringWithColor(fmt.Sprintf("Enchantment: +%d", s.Plus), defs.OffWhite),
		util.LabelTextFromStringWithColor(fmt.Sprintf("Durability: %s", s.Durability.ToString()), defs.OffWhite),
		util.LabelTextFromStringWithColor(fmt.Sprintf("Attack Speed: %.2f", s.AttackSpeedFactor), defs.OffWhite),
	}

	var requirementsInfo []util.LabelText
	caser := cases.Title(language.English)
	if len(s.Requirements) > 0 {
		for attr, value := range s.Requirements {
			var reqColor color.Color
			reqColor = defs.DarkTextColor
			if forActor.GetStats().GetAttribute(attr) < value {
				reqColor = defs.NegativeHighlightColor
			}
			niceNameForAttribute := caser.String(strings.Replace(string(attr), "_", " ", -1))
			requirementsInfo = append(requirementsInfo, util.LabelText{
				Text:      fmt.Sprintf("%s: %d", niceNameForAttribute, value),
				TextColor: reqColor,
			})
		}
	}

	if len(requirementsInfo) > 0 {
		baseInfo = append(baseInfo, util.LabelTextFromString(""), util.LabelTextFromString("Requirements"))
		baseInfo = append(baseInfo, requirementsInfo...)
	}

	return baseInfo
}
func (s WeaponStats) ToTableString() []string {
	baseInfo := []util.TableRow{
		{Label: "Type:", Columns: []string{s.Type.ToString()}},
		{Label: "Damage:", Columns: []string{s.Damage.String()}},
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
func (s WeaponStats) IsMeleeWeapon() bool {
	return s.Type == defs.WeaponTypeSword ||
		s.Type == defs.WeaponTypeAxe ||
		s.Type == defs.WeaponTypeMace ||
		s.Type == defs.WeaponTypeScythe ||
		s.Type == defs.WeaponTypeSpear ||
		s.Type == defs.WeaponTypeDagger
}

func (s WeaponStats) IsRangedWeapon() bool {
	return s.Type == defs.WeaponTypeBow ||
		s.Type == defs.WeaponTypeCrossbow ||
		s.Type == defs.WeaponTypeSling
}

func (s WeaponStats) NameParts() string {
	// angband style (3, +3)
	return fmt.Sprintf("(%s, +%d)", s.Damage.ShortString(), s.Plus)
}

func (s WeaponStats) Clone() *WeaponStats {
	return &WeaponStats{
		Damage:                      s.Damage,
		Requirements:                s.Requirements,
		Durability:                  stats.NewResource(s.Durability.Min(), s.Durability.GetMax, s.Durability.Value()),
		Type:                        s.Type,
		AttackSpeedFactor:           s.AttackSpeedFactor,
		MeleeRange:                  s.MeleeRange,
	}
}

func (s WeaponStats) CanBeFurtherEnchanted() bool {
	return s.Plus < 3
}

func (s WeaponStats) GetInventorySlot() defs.SlotName {
	if s.IsRangedWeapon(){
		return defs.SlotNameMissileLauncher
	}
	if s.Type == defs.WeaponTypeShield {
		return defs.SlotNameOffHand
	}
	if s.Type == defs.WeaponTypeScythe || s.Type == defs.WeaponTypeSpear {
		return defs.SlotNameTwoHandedWeapon
	}
	if s.IsTwoHanded {
		return defs.SlotNameTwoHandedWeapon
	}
	return defs.SlotNameMainHand
}