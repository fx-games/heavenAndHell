package common

import (
	"fmt"
	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
)

func (i *Item) D1() *d1.Item { return i.d1 }

// Decision 8: one icon per D1 item class, with dedicated IconItemMagic*/IconItemUnique* icons for quality (items.rec) -
// items are never tinted on the map.
var d1ITypeNames = []string{"ITYPE_MISC", "ITYPE_SWORD", "ITYPE_AXE", "ITYPE_BOW", "ITYPE_MACE", "ITYPE_SHIELD", "ITYPE_LARMOR",
	"ITYPE_HELM", "ITYPE_MARMOR", "ITYPE_HARMOR", "ITYPE_STAFF", "ITYPE_GOLD", "ITYPE_RING", "ITYPE_AMULET"}

// d1RarityIconSuffix names the IconItemMagic<suffix>/IconItemUnique<suffix> icon for every D1 item type that rolls
// affixes or uniques; magic items are never tinted, so each needs one in items.rec.
var d1RarityIconSuffix = map[int]string{
	d1.ITypeSword: "Sword", d1.ITypeAxe: "Axe", d1.ITypeMace: "Mace", d1.ITypeBow: "Bow", d1.ITypeStaff: "Staff",
	d1.ITypeShield: "Shield", d1.ITypeLArmor: "Armor", d1.ITypeMArmor: "Armor", d1.ITypeHArmor: "Armor", d1.ITypeHelm: "Helmet",
	d1.ITypeRing: "Ring", d1.ITypeAmulet: "Amulet",
}

// d1ItemIcon: icons.rec "item.IMISC_*" if present (potions, scrolls), else the magic/unique variant of the item's
// class icon if it.Magical calls for one, else "item.ITYPE_*".
func d1ItemIcon(it *d1.Item) int32 {
	return defs.Icon(d1ItemIconName(it))
}

func d1ItemIconName(it *d1.Item) string {
	if _, ok := defs.IconOK("item." + it.MiscID); ok {
		return "item." + it.MiscID
	}
	if suffix, ok := d1RarityIconSuffix[it.Type]; ok && it.Magical > 0 {
		prefix := "IconItemMagic"
		if it.Magical == 2 {
			prefix = "IconItemUnique"
		}
		return prefix + suffix
	}
	return "item." + d1ITypeNames[it.Type]
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
	item := NewItem(g.State.GetPlayer, it.Name, d1ItemIcon(it))
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
		if it.Type == d1.ITypeSword || it.Type == d1.ITypeAxe || it.Type == d1.ITypeMace {
			item.SetThrowComponent(&ThrowStats{
				Damage:        stats.NewInterval(it.MinDam, it.MaxDam),
				ChanceToBreak: 0.25,
				HitZone:       OneTargetTile(g.State.GetMap, nil),
			})
		}
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
			item.SetCategory(defs.ItemCategoryScroll)
			item.SetCharges(1)
			item.SetDestroyAfterUse(true)
			g.d1SpellItemAction(item)
		case "IMISC_BOOK":
			book := it
			action := NewUntargetedAction(it.Name, item.icon)
			action.SetEffect(&UntargetedEffect{
				Effect:      func(user *Actor, _ EffectSource) { g.readD1Book(user, book) },
				Description: []util.LabelText{util.LabelTextFromString(it.Name)},
			})
			action.SetCondition(func(user *Actor) bool {
				if user.HeroSheet().Mag < book.MinMag {
					g.Msg(fmt.Sprintf("You need %d magic to read that.", book.MinMag))
					return false
				}
				return true
			})
			item.SetCategory(defs.ItemCategoryScroll)
			item.SetCharges(1)
			item.SetDestroyAfterUse(true)
			item.SetUntargetedAction(action)
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
	if it.Type == d1.ITypeStaff && it.Spell > 0 && it.MaxCharges > 0 {
		g.d1SpellItemAction(item)
	}
	item.SetRarity(ItemRarity(it.Magical))
	item.RefreshD1()
	return item
}

// MeetsRequirements reports whether the wielding player's Str/Mag/Dex meet this D1 item's requirements. Always
// true for non-D1 items.
func (i *Item) MeetsRequirements() bool {
	if i.d1 == nil {
		return true
	}
	it := i.d1
	player := i.getPlayer()
	if player == nil {
		return true
	}
	s := player.GetStats()
	return s.GetAttribute(stats.Strength) >= it.MinStr && s.GetAttribute(stats.Magic) >= it.MinMag && s.GetAttribute(stats.Dexterity) >= it.MinDex
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
	if it.Spell > 0 && it.Spell < len(d1.SpellDescriptions) {
		lines = append(lines, d1.SpellDescriptions[it.Spell])
	}
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
		if i.HasAction() {
			i.actionCharges = it.Charges
		}
	}
	if it.Magical > 0 && !it.Identified {
		add("Not Identified")
	} else {
		lines = append(lines, it.Properties()...)
	}
	if it.Type == d1.ITypeSword || it.Type == d1.ITypeAxe || it.Type == d1.ITypeMace || it.Type == d1.ITypeBow || it.Type == d1.ITypeStaff {
		if i.SlotName() == defs.SlotNameTwoHandedWeapon {
			add("Two-Handed (cannot use a shield)")
		} else {
			add("One-Handed (a shield can be worn)")
		}
	}
	if it.MinStr+it.MinMag+it.MinDex > 0 {
		player := i.getPlayer()
		unmet := func(attr stats.Name, need int) string {
			if player != nil && player.GetStats().GetAttribute(attr) < need {
				return "[:red]"
			}
			return ""
		}
		add("Req:%s %d Str[:white] %s %d Mag[:white] %s %d Dex[:white]",
			unmet(stats.Strength, it.MinStr), it.MinStr, unmet(stats.Magic, it.MinMag), it.MinMag, unmet(stats.Dexterity, it.MinDex), it.MinDex)
	}
	if player := i.getPlayer(); player != nil && player.hero != nil {
		h, sh := player.hero, player.HeroSheet()
		life := func() { lo, hi := h.PotionHealRange(sh.MaxHP); add("Restores %d-%d life", lo>>6, hi>>6) }
		mana := func() { lo, hi := h.PotionManaRange(sh.MaxMana); add("Restores %d-%d mana", lo>>6, hi>>6) }
		switch it.MiscID {
		case "IMISC_HEAL":
			life()
		case "IMISC_MANA":
			mana()
		case "IMISC_REJUV":
			life()
			mana()
		case "IMISC_FULLHEAL":
			add("Restores all life")
		case "IMISC_FULLMANA":
			add("Restores all mana")
		case "IMISC_FULLREJUV":
			add("Restores all life and mana")
		}
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
