package common

import (
	"fmt"
	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

func (i *Item) D1() *d1.Item { return i.d1 }

// Decision 8: one icon per D1 item class, quality shown by rarity tint. First guesses; tune freely.
var d1TypeIcons = map[int]int32{
	d1.ITypeSword: 646, d1.ITypeAxe: 697, d1.ITypeMace: 659, d1.ITypeBow: 621, d1.ITypeStaff: defs.IconItemStaff,
	d1.ITypeShield: 204, d1.ITypeLArmor: 227, d1.ITypeHelm: 211, d1.ITypeMArmor: 230, d1.ITypeHArmor: 199,
	d1.ITypeRing: 129, d1.ITypeAmulet: 132,
	d1.ITypeGold: 273, // gold pile
}

var d1MiscIcons = map[string]int32{
	"IMISC_HEAL": defs.IconItemPotionRed, "IMISC_FULLHEAL": defs.IconItemPotionCrimson,
	"IMISC_MANA": defs.IconItemPotionBlue, "IMISC_FULLMANA": defs.IconItemPotionDarkBlue,
	"IMISC_REJUV": defs.IconItemPotionPink, "IMISC_FULLREJUV": defs.IconItemPotionWhite,
	"IMISC_ELIXSTR": defs.IconItemPotionOrange, "IMISC_ELIXMAG": defs.IconItemPotionOrange,
	"IMISC_ELIXDEX": defs.IconItemPotionOrange, "IMISC_ELIXVIT": defs.IconItemPotionOrange,
	"IMISC_SPECELIX": defs.IconItemPotionGreen,
	"IMISC_SCROLL":   defs.IconItemScroll, "IMISC_SCROLLT": defs.IconItemScroll, "IMISC_BOOK": defs.IconItemScroll,
}

func d1ItemIcon(it *d1.Item) int32 {
	if icon, ok := d1MiscIcons[it.MiscID]; ok {
		return icon
	}
	if icon, ok := d1TypeIcons[it.Type]; ok {
		return icon
	}
	return defs.IconItemScroll
}

func d1SlotName(it *d1.Item) defs.SlotName {
	switch it.Loc {
	case "ILOC_ONEHAND":
		if it.Type == d1.ITypeShield {
			return defs.SlotNameOffHand
		}
		return defs.SlotNameMainHand
	case "ILOC_TWOHAND":
		return defs.SlotNameTwoHandedWeapon
	case "ILOC_ARMOR":
		return defs.SlotNameArmorTorso
	case "ILOC_HELM":
		return defs.SlotNameArmorHead
	case "ILOC_RING":
		return defs.SlotNameRing
	case "ILOC_AMULET":
		return defs.SlotNameAmulet
	}
	return defs.SlotNameNotEquippable
}

// NewD1Item wraps a rolled D1 item into a game item; the D1 fields stay the source of truth.
func (g *Factory) NewD1Item(it *d1.Item) *Item {
	item := NewItem(g.state.GetPlayer, it.Name, d1ItemIcon(it))
	item.d1 = it
	item.SetInternalName(it.Name)
	item.SetBaseCost(it.IValue)
	durability := stats.NewResource(0, flatFunc(it.MaxDur), it.Durability)
	switch it.Type {
	case d1.ITypeSword, d1.ITypeAxe, d1.ITypeMace, d1.ITypeBow, d1.ITypeStaff:
		item.SetCategory(defs.ItemCategoryWeapon)
		weaponType := map[int]defs.WeaponType{d1.ITypeSword: defs.WeaponTypeSword, d1.ITypeAxe: defs.WeaponTypeAxe,
			d1.ITypeMace: defs.WeaponTypeMace, d1.ITypeBow: defs.WeaponTypeBow, d1.ITypeStaff: defs.WeaponTypeSpear}[it.Type]
		item.SetWeaponComponent(&WeaponStats{
			Damage: util.NewDice(1, max(it.MaxDam-it.MinDam+1, 1), it.MinDam-1), Durability: durability,
			Type: weaponType, AttackSpeedFactor: 1, MeleeRange: 1, MaxRange: 12,
			AccuracyAtMinRange: 1, AccuracyAtMaxRange: 1, IsTwoHanded: it.Loc == "ILOC_TWOHAND", ProjectileIcon: -1,
		})
	case d1.ITypeShield, d1.ITypeLArmor, d1.ITypeMArmor, d1.ITypeHArmor, d1.ITypeHelm:
		item.SetCategory(defs.ItemCategoryArmor)
		armorType := defs.ArmorTypeCuirass
		switch it.Type {
		case d1.ITypeShield:
			armorType = defs.ArmorTypeShield
		case d1.ITypeHelm:
			armorType = defs.ArmorTypeHelmet
		}
		item.SetArmorComponent(&ArmorStats{BaseArmor: it.AC, Type: armorType, Durability: durability})
	case d1.ITypeRing:
		item.SetCategory(defs.ItemCategoryRing)
	case d1.ITypeAmulet:
		item.SetCategory(defs.ItemCategoryAmulet)
	default:
		item.SetCategory(defs.ItemCategoryOther)
		switch it.MiscID {
		case "IMISC_SCROLL", "IMISC_SCROLLT":
			item.SetCategory(defs.ItemCategoryScroll) // ponytail: reading scrolls comes with spells (milestone 5)
		case "IMISC_HEAL", "IMISC_FULLHEAL", "IMISC_MANA", "IMISC_FULLMANA", "IMISC_REJUV", "IMISC_FULLREJUV",
			"IMISC_ELIXSTR", "IMISC_ELIXMAG", "IMISC_ELIXDEX", "IMISC_ELIXVIT", "IMISC_SPECELIX":
			miscID := it.MiscID
			action := NewUntargetedAction(it.Name, item.icon)
			action.SetEffect(&UntargetedEffect{
				Effect:      func(user *Actor, _ EffectSource) { UseD1Potion(user, miscID) },
				Description: []util.LabelText{util.LabelTextFromString(it.Name)},
			})
			item.SetCategory(defs.ItemCategoryPotion)
			item.SetCharges(1)
			item.SetDestroyAfterUse(true)
			item.SetUntargetedAction(action)
		}
	}
	item.SetRarity(ItemRarity(it.Magical))
	item.RefreshD1()
	return item
}

// RefreshD1 updates name and tooltip from the D1 item (e.g. after identification).
func (i *Item) RefreshD1() {
	it := i.d1
	i.name = it.Name
	if it.Identified {
		i.name = it.IName
	}
	if it.Type == d1.ITypeGold {
		i.name = fmt.Sprintf("%d gold", it.Value)
	}
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	if it.MaxDam > 0 {
		add("Damage: %d-%d", it.MinDam, it.MaxDam)
	}
	if it.AC > 0 {
		add("Armor: %d", it.AC)
	}
	if it.MaxDur > 0 && it.MaxDur < 255 {
		add("Durability: %d/%d", it.Durability, it.MaxDur)
	}
	if it.MaxCharges > 0 {
		add("Charges: %d/%d", it.Charges, it.MaxCharges)
	}
	if it.Magical > 0 && !it.Identified {
		add("Not Identified")
	} else {
		lines = append(lines, it.Properties()...)
	}
	if it.MinStr+it.MinMag+it.MinDex > 0 {
		add("Required: %d Str, %d Mag, %d Dex", it.MinStr, it.MinMag, it.MinDex)
	}
	value := it.IValue
	if !it.Identified {
		value = it.Value
	}
	if it.Type != d1.ITypeGold {
		add("Value: %d gold", value)
	}
	i.forcedTooltip = util.ToLabelText(append([]string{""}, lines...))
}
