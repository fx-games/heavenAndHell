package common

import (
	"fmt"
	"strconv"
	"strings"

	"rpg/src/defs"
	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
)

func (i *Item) Roll() *rules.Item { return i.roll }

// itemIcon: icons.rec "item.<misc id>" if present (potions, scrolls), else the ".magic"/".unique" variant of the
// item's own family icon if it.Magical calls for one, else the family icon, else "item.<type>".
// The last step of the chain is the base record's own "icon:" field, for a mod that names its items' icons
// outright instead of leaning on Diablo's ITYPE_/IMISC_ enums. D1's own names all resolve before this, so its
// items are unaffected.
func (g *Factory) itemIcon(it *rules.Item) int32 {
	if icon, ok := defs.IconOK(itemIconName(it)); ok {
		return icon
	}
	if it.Base != nil {
		if icon, ok := defs.IconOK(it.Base.Icon); ok {
			return icon
		}
	}
	return defs.Icon(itemIconName(it)) // no icon anywhere: fail naming the one that is missing
}

// goldIconName picks the gold pile icon by thirds of the spawned range: 5*dlvl + rand(10*dlvl), +1/8 in hell,
// dlvl 1..16 gives 5..268.
func goldIconName(value int) string {
	const lo, hi = 5, 268
	switch {
	case value < lo+(hi-lo)/3:
		return "item.gold_small"
	case value < lo+2*(hi-lo)/3:
		return "item.gold_medium"
	}
	return "item.gold_large"
}

func itemIconName(it *rules.Item) string {
	if it.Type == rules.ITypeGold {
		return goldIconName(it.Value)
	}
	if id := "item." + it.Unique; it.Unique != "" {
		if _, ok := defs.IconOK(id); ok { // a unique with an icon of its own
			return id
		}
	}
	familyName := defs.IconID("item", it.Name) // quest items and every weapon/armor family are keyed by base name
	if it.Magical > 0 {
		suffix := ".magic"
		if it.Magical == 2 {
			suffix = ".unique"
		}
		if _, ok := defs.IconOK(familyName + suffix); ok {
			return familyName + suffix
		}
	}
	if _, ok := defs.IconOK(familyName); ok {
		return familyName
	}
	miscName := defs.IconID("item", it.MiscID)
	if _, ok := defs.IconOK(miscName); ok { // potions/scrolls/runes/oils, keyed by IMISC_*
		return miscName
	}
	return "item." + defs.ItemTypeNames[it.Type] // one icon per item type is the last resort
}

// slotName is the slot an item goes in; dualWield (the Bard) puts a one-handed weapon in either free hand,
// which SlotNameOneHandedWeapon stands for the way SlotNameRing stands for either ring finger.
func slotName(it *rules.Item, dualWield bool) defs.SlotName {
	switch it.Loc {
	case "ILOC_ONEHAND":
		if it.Type == rules.ITypeShield {
			return defs.SlotNameOffHand
		}
		if dualWield {
			return defs.SlotNameOneHandedWeapon
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
	case "ILOC_FEET":
		return defs.SlotNameArmorFeet
	case "ILOC_HANDS":
		return defs.SlotNameArmorHands
	case "ILOC_BACK":
		return defs.SlotNameArmorBack
	case "ILOC_LIGHT":
		return defs.SlotNameLightSource
	}
	return defs.SlotNameNotEquippable
}

// NewRolledItem wraps a rolled D1 item into a game item; the D1 fields stay the source of truth.
func (g *Factory) NewRolledItem(it *rules.Item) *Item {
	g.Generator.D.Relink(it) // a loaded item's base
	item := NewItem(g.State.GetPlayer, it.Name, g.itemIcon(it))
	item.roll, item.data = it, g.Generator.D
	item.SetInternalName(it.Name)
	item.SetBaseCost(it.IValue)
	durability := stats.NewResource(0, flatFunc(it.MaxDur), it.Durability)
	switch it.Type {
	case rules.ITypeSword, rules.ITypeAxe, rules.ITypeMace, rules.ITypeBow, rules.ITypeStaff:
		item.SetCategory(defs.ItemCategoryWeapon)
		weaponType := map[int]defs.WeaponType{rules.ITypeSword: defs.WeaponTypeSword, rules.ITypeAxe: defs.WeaponTypeAxe,
			rules.ITypeMace: defs.WeaponTypeMace, rules.ITypeBow: defs.WeaponTypeBow, rules.ITypeStaff: defs.WeaponTypeSpear}[it.Type]
		item.SetWeaponComponent(&WeaponStats{
			Damage: util.NewDice(1, max(it.MaxDam-it.MinDam+1, 1), it.MinDam-1), Durability: durability,
			Type: weaponType, AttackSpeedFactor: 1, MeleeRange: 1, MaxRange: 12,
			AccuracyAtMinRange: 1, AccuracyAtMaxRange: 1, IsTwoHanded: it.Loc == "ILOC_TWOHAND", ProjectileIcon: -1,
		})
		if it.Type == rules.ITypeSword || it.Type == rules.ITypeAxe || it.Type == rules.ITypeMace {
			item.SetThrowComponent(&ThrowStats{
				Damage:        util.NewInterval(it.MinDam, it.MaxDam),
				ChanceToBreak: 0.25,
				HitZone:       OneTargetTile(g.State.GetMap, nil),
			})
		}
	case rules.ITypeShield, rules.ITypeLArmor, rules.ITypeMArmor, rules.ITypeHArmor, rules.ITypeHelm:
		item.SetCategory(defs.ItemCategoryArmor)
		armorType := defs.ArmorTypeCuirass
		switch it.Type {
		case rules.ITypeShield:
			armorType = defs.ArmorTypeShield
		case rules.ITypeHelm:
			armorType = defs.ArmorTypeHelmet
		}
		item.SetArmorComponent(&ArmorStats{BaseArmor: it.AC, Type: armorType, Durability: durability})
		item.SetThrowComponent(&ThrowStats{ // house rule, D1 has no throwing
			Damage:        util.NewInterval(0, it.AC),
			ChanceToBreak: 0.25,
			HitZone:       OneTargetTile(g.State.GetMap, nil),
		})
	case rules.ITypeRing:
		item.SetCategory(defs.ItemCategoryRing)
	case rules.ITypeAmulet:
		item.SetCategory(defs.ItemCategoryAmulet)
	default:
		item.SetCategory(defs.ItemCategoryOther)
		switch it.MiscID {
		case "IMISC_SCROLL", "IMISC_SCROLLT":
			item.SetCategory(defs.ItemCategoryScroll)
			item.SetCharges(1)
			item.SetDestroyAfterUse(true)
			g.spellItemAction(item)
		case "IMISC_BOOK":
			book := it
			action := NewUntargetedAction(it.Name, item.icon)
			action.SetEffect(&UntargetedEffect{
				Effect:      func(user *Actor, _ EffectSource, _ bool) { g.readBook(user, book) },
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
				Effect:      func(user *Actor, _ EffectSource, _ bool) { UsePotion(user, miscID) },
				Description: []util.LabelText{util.LabelTextFromString(it.Name)},
			})
			item.SetCategory(defs.ItemCategoryPotion)
			item.SetCharges(1)
			item.SetDestroyAfterUse(true)
			item.SetUntargetedAction(action)
		default:
			if g.Generator.D.IsOil(it.MiscID) { // Hellfire's oils: pick an item out of the pack and pour it on
				g.oilAction(item, it.MiscID, it.Name)
			}
			g.runeAction(item, it)
		}
	}
	if it.Type == rules.ITypeStaff && it.Spell > 0 && it.MaxCharges > 0 {
		g.spellItemAction(item)
	}
	if it.Base != nil {
		g.applyItemRecord(item, *it.Base)
	}
	item.SetRarity(ItemRarity(it.Magical))
	item.Refresh()
	return item
}

func (i *Item) IsGold() bool { return i.roll != nil && i.roll.Type == rules.ITypeGold }

// MeetsRequirements reports whether the wielding player's Str/Mag/Dex meet this D1 item's requirements. Always
// true for non-D1 items.
func (i *Item) MeetsRequirements() bool {
	if i.roll == nil {
		return true
	}
	it := i.roll
	player := i.getPlayer()
	if player == nil {
		return true
	}
	s := player.GetStats()
	return s.GetAttribute(stats.Strength) >= it.MinStr && s.GetAttribute(stats.Magic) >= it.MinMag && s.GetAttribute(stats.Dexterity) >= it.MinDex
}

// Refresh updates name and tooltip from the D1 item (e.g. after identification).
func (i *Item) Refresh() {
	it := i.roll
	i.name = it.Name
	if it.Identified {
		i.name = it.IName
	}
	if it.Type == rules.ITypeGold {
		i.name = fmt.Sprintf("%d gold", it.Value)
	}
	if it.MaxCharges > 0 && i.HasAction() {
		i.actionCharges = it.Charges
	}
	i.forcedTooltip = util.ToLabelText(append([]string{""}, i.tooltipLines(nil)...))
}

// TooltipLinesComparedTo is GetTooltipLines with Damage, Armor and property lines colored by cmp (rules.Item.Compare):
// green where this item is better, red where it is worse.
func (i *Item) TooltipLinesComparedTo(cmp map[string]int, parseColorCodedText func(string) []util.ColoredTextPart) []util.LabelText {
	i.Refresh()
	lines := util.ToLabelText(append([]string{i.Name(), ""}, i.tooltipLines(cmp)...))
	lines[0].TextColor = defs.OffWhite
	return parseColorCodes(lines, parseColorCodedText)
}

func (i *Item) tooltipLines(cmp map[string]int) []string {
	it := i.roll
	var lines []string
	add := func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }
	colored := func(c int, text string) string {
		switch c {
		case 1:
			return "[:positive]" + text + "[:white]"
		case -1:
			return "[:negative]" + text + "[:white]"
		}
		return text
	}
	if it.Spell > 0 && it.Spell < len(i.data.Spells) {
		lines = append(lines, i.data.Spells[it.Spell].Description)
		// what the spell does for this hero, right under what it is (a staff's own melee damage follows below)
		if player := i.getPlayer(); player != nil && player.hero != nil {
			h, sh := player.hero, player.HeroSheet()
			if it.Spell == rules.SplHeal {
				lo, hi := h.Rules().HealRange64(h.Class, h.Level, h.SpellLevel(sh, it.Spell))
				add("Spell heals %d-%d life", rules.Whole(lo), rules.Whole(hi))
			}
			if t := i.data.SpellDamageText(it.Spell, sh.Mag, h.Level, h.SpellLevel(sh, it.Spell)); t != "" {
				add("Spell: %s", t)
			}
		}
	}
	// what an H&H potion or scroll does: its effect's own description (a spell's action repeats the spell's, shown above)
	var desc []util.LabelText
	if i.targetedAction != nil && it.Spell == 0 {
		desc = i.targetedAction.GetDescription()
	}
	if i.untargetedAction != nil && it.Spell == 0 {
		desc = append(desc, i.untargetedAction.GetDescription()...)
	}
	for _, l := range desc {
		lines = append(lines, l.Text)
	}
	switch it.MiscID {
	case "IMISC_OILACC":
		add("Increases a weapon's chance to hit by 1-2%%")
	case "IMISC_OILMAST":
		add("Increases a weapon's chance to hit by 3-5%%")
	case "IMISC_OILSHARP":
		add("Adds 1 to a weapon's maximum damage")
	case "IMISC_OILDEATH":
		add("Adds 1-2 to a weapon's minimum and maximum damage")
	case "IMISC_OILSKILL":
		add("Lowers an item's Str, Mag and Dex requirements by 5-10")
	case "IMISC_OILBSMTH":
		add("Repairs 20%% of an item's durability")
	case "IMISC_OILFORT":
		add("Adds 10-50 to an item's max durability")
	case "IMISC_OILPERM":
		add("Makes an item indestructible")
	case "IMISC_OILHARD":
		add("Adds 1-2 Armor Class to a helm, shield or armor")
	case "IMISC_OILIMP":
		add("Adds 3-5 Armor Class to armor or a shield")
	case "IMISC_OILOF":
		add("Applies a random enchantment to an item in your pack")
	}
	if it.MaxDam > 0 {
		add("%s", colored(cmp[rules.KeyDamage], fmt.Sprintf("Damage: %d-%d", it.MinDam, it.MaxDam)))
	}
	if it.AC > 0 {
		add("%s", colored(cmp[rules.KeyArmor], fmt.Sprintf("Armor: %d", it.AC)))
	}
	// what the weapon's kind does on its own, before any affix: the mod's weapon_bonus rules (D1: PlrHitMonst's)
	for _, b := range i.data.WeaponBonuses {
		if b.Weapon == it.Type {
			tone := "[:positive]+"
			if b.Percent < 0 {
				tone = "[:negative]"
			}
			add("%s%d%% damage vs. %s[:white]", tone, b.Percent, monsterClassName(b.Versus))
		}
	}
	if it.MaxDur > 0 && it.MaxDur < 255 {
		add("Durability: %d/%d", it.Durability, it.MaxDur)
		if it.Durability == 0 {
			add("[:red]Broken: no effect until repaired[:white]")
		}
	}
	if it.MaxCharges > 0 {
		add("Charges: %d/%d", it.Charges, it.MaxCharges)
	}
	if it.Magical > 0 && !it.Identified {
		add("Not Identified")
	} else {
		for _, l := range i.data.PropertyLines(it, cmp) {
			if l.Cmp == 0 { // affixes stand out from the base stats; a comparison's green/red wins over the blue
				lines = append(lines, "[:magic]"+l.Text+"[:white]")
				continue
			}
			lines = append(lines, colored(l.Cmp, l.Text))
		}
	}
	if it.Type == rules.ITypeSword || it.Type == rules.ITypeAxe || it.Type == rules.ITypeMace || it.Type == rules.ITypeBow || it.Type == rules.ITypeStaff {
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
		life := func() {
			lo, hi := h.PotionHealRange(sh.MaxHP)
			add("Restores %d-%d life", rules.Whole(lo), rules.Whole(hi))
		}
		mana := func() {
			lo, hi := h.PotionManaRange(sh.MaxMana)
			add("Restores %d-%d mana", rules.Whole(lo), rules.Whole(hi))
		}
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
	if it.Type != rules.ITypeGold {
		add("Value: %d gold", value)
	}
	return lines
}

// NoteUnequipped marks an equipped item that equipping another would take off without a replacement.
const NoteUnequipped = "Would be unequipped"

// EquippedComparison is an equipped item equipping another item would displace, with its values compared to the other
// item's (Cmp, see rules.Item.Compare) and an optional note.
type EquippedComparison struct {
	Item *Item
	Cmp  map[string]int
	Note string
}

// CompareForEquipping lists the equipped items candidate (a shop or pack item) would replace (left ring first, weapon before shield) and how
// candidate's values compare to them: per key better or worse than all of them, else 0. Empty if nothing is displaced.
func (e *Equipment) CompareForEquipping(candidate *Item) ([]EquippedComparison, map[string]int) {
	if candidate.roll == nil {
		return nil, nil
	}
	var result []EquippedComparison
	var candidateCmps []map[string]int
	compare := func(worn *Item) {
		if worn == nil || worn.roll == nil {
			return
		}
		wornKnown := !(worn.roll.Magical > 0 && !worn.roll.Identified)
		known := wornKnown && !(candidate.roll.Magical > 0 && !candidate.roll.Identified)
		c := EquippedComparison{Item: worn, Cmp: worn.data.Compare(worn.roll, candidate.roll, known)}
		if !wornKnown { // an unidentified candidate says so in its own tooltip
			c.Note = "Affixes unknown (not identified)"
		}
		result = append(result, c)
		candidateCmps = append(candidateCmps, candidate.data.Compare(candidate.roll, worn.roll, known))
	}
	mainHand := e.GetBySlot(defs.SlotNameMainHand)
	switch candidate.SlotName() {
	case defs.SlotNameNotEquippable:
		return nil, nil
	case defs.SlotNameRing:
		compare(e.GetBySlot(defs.SlotNameRingLeft))
		compare(e.GetBySlot(defs.SlotNameRingRight))
	case defs.SlotNameTwoHandedWeapon:
		compare(mainHand)
		if shield := e.GetBySlot(defs.SlotNameOffHand); shield != nil && shield.roll != nil {
			lost := shield.data.Compare(shield.roll, nil, !(shield.roll.Magical > 0 && !shield.roll.Identified))
			for key := range lost {
				lost[key] = -lost[key] // everything the shield gives is lost
			}
			result = append(result, EquippedComparison{Item: shield, Cmp: lost})
		}
	case defs.SlotNameOffHand:
		if mainHand != nil && mainHand.SlotName() == defs.SlotNameTwoHandedWeapon {
			return []EquippedComparison{{Item: mainHand, Note: NoteUnequipped}}, nil
		}
		compare(e.GetBySlot(defs.SlotNameOffHand))
	case defs.SlotNameOneHandedWeapon: // a dual wielder fills a free hand, so nothing is replaced until both are full
		if e.isOneSlotAvailable(defs.SlotNameMainHand, defs.SlotNameOffHand) {
			break
		}
		compare(mainHand)
	default:
		compare(e.GetBySlot(candidate.SlotName()))
	}

	candidateCmp := map[string]int{}
	for i, c := range candidateCmps {
		for key, v := range c {
			if i == 0 {
				candidateCmp[key] = v
			} else if candidateCmp[key] != v {
				candidateCmp[key] = 0
			}
		}
	}
	return result, candidateCmp
}

// applyItemRecord gives an item what its record declares beyond the D1 fields: its category, the effect it has on
// use, its light, its ranged and throwing numbers. Only records that name a category (H&H's) carry any of it.
func (g *Factory) applyItemRecord(item *Item, b rules.ItemData) {
	if b.InventoryCategory == defs.ItemCategoryInvalid {
		return
	}
	item.SetCategory(b.InventoryCategory)
	if w := item.weaponComponent; w != nil && b.Weapon.IsValid() {
		w.Damage, w.AttackSpeedFactor, w.ArmorPenetration = b.Weapon.Damage, max(b.Weapon.Speed, 0.1), b.Weapon.ArmorPenetration
		if b.Weapon.MaxRange > 0 {
			w.MaxRange, w.AccuracyAtMinRange, w.AccuracyAtMaxRange = b.Weapon.MaxRange, b.Weapon.AccuracyAtMinRange, b.Weapon.AccuracyAtMaxRange
		}
		w.ProjectileIcon = b.Weapon.ProjectileIcon
	}
	if b.Throw.IsValid() {
		if zone := HitZoneFromString(g.State.GetMap, b.Throw.HitZone); zone != nil {
			item.SetThrowComponent(&ThrowStats{Damage: b.Throw.Damage, HitZone: zone, ChanceToBreak: b.Throw.ChanceToBreak,
				OnHitEffect: g.NewOnHitEffectFromString(b.Throw.OnHitEffect)})
		}
	}
	if b.InventoryCategory == defs.ItemCategoryLightSource {
		light := g.State.aura()
		if b.Light.IsValid() {
			light = LightStats{LightColor: util.GetColorFromRGBString(b.Light.LightColor), FlickerPattern: defs.FlickerPattern(b.Light.FlickerPattern),
				FlickerFrameDelay: b.Light.FlickerFrameDelay}
		}
		item.SetLightComponent(&light)
	}
	if b.UntargetedAction != "" {
		if action := g.NewUntargetedActionFromName(b.UntargetedAction); action != nil {
			item.SetUntargetedAction(action)
		}
	}
	if b.TargetedAction != "" {
		if action := g.NewTargetedActionFromName(b.TargetedAction); action != nil {
			item.SetTargetedAction(action)
		}
	}
	switch {
	case b.Charges < 0:
		item.SetUnlimitedCharges()
	case b.InventoryCategory.HasAlwaysOneCharge():
		item.SetCharges(1)
		item.SetDestroyAfterUse(true)
	case b.Charges != 0:
		item.SetCharges(b.Charges)
	}
	for _, m := range b.Modifiers { // what the hero sheet has no power for, the actor's stats carry while equipped
		name, arg, _ := strings.Cut(strings.TrimSuffix(m, ")"), "(")
		lvl := max(b.DLvl, 1) // speed(n) rolls up to n, a bare speed up to the item's dlvl
		if n, err := strconv.Atoi(strings.TrimSpace(arg)); err == nil {
			lvl = n
		}
		switch name {
		case "speed":
			item.AddEquippedModifier(defs.AddToBaseAttributeRandomly(g.RandomSource, stats.BaseSpeed, util.NewTuple(1, lvl)))
		case "move_speed":
			item.AddEquippedModifier(&stats.Modifier{Attribute: stats.MoveSpeedFactor, Description: "of running",
				Value: g.RandomSource.Float64() * float64(lvl) * 0.1, Mode: stats.ModifyAdditive, IsPercentBased: true})
		}
	}
	if b.Hidden {
		item.Hide()
		item.ShowNameOnlyInTooltip()
	}
}

func monsterClassName(class string) string {
	switch class {
	case "MC_UNDEAD":
		return "the undead"
	case "MC_ANIMAL":
		return "animals"
	case "MC_DEMON":
		return "demons"
	}
	return class
}
