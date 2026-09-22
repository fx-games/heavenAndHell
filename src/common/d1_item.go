package common

import (
	"fmt"
	"rpg/src/d1"
	"rpg/src/defs"
	"rpg/src/stats"
	"rpg/src/util"
)

func (i *Item) D1() *d1.Item { return i.d1 }

// Decision 8: one icon per D1 weapon/armor family (falchion, dagger, broad sword...), with a ".magic"/".unique"
// variant of each for quality (items.rec) - items are never tinted on the map.
var d1ITypeFallback = []string{"item.misc", "item.sword", "item.axe", "item.bow", "item.mace", "item.shield",
	"item.larmor", "item.helm", "item.marmor", "item.harmor", "item.staff", "item.gold", "item.ring", "item.amulet"}

// d1ItemIcon: icons.rec "item.<misc id>" if present (potions, scrolls), else the ".magic"/".unique" variant of the
// item's own family icon if it.Magical calls for one, else the family icon, else "item.<type>".
// The last step of the chain is the base record's own "icon:" field, for a mod that names its items' icons
// outright instead of leaning on Diablo's ITYPE_/IMISC_ enums. D1's own names all resolve before this, so its
// items are unaffected.
func (g *Factory) d1ItemIcon(it *d1.Item) int32 {
	if icon, ok := defs.IconOK(d1ItemIconName(it)); ok {
		return icon
	}
	if it.BaseIdx >= 0 && it.BaseIdx < len(g.Generator.D.Items) {
		if icon, ok := defs.IconOK(g.Generator.D.Items[it.BaseIdx].Icon); ok {
			return icon
		}
	}
	return defs.Icon(d1ItemIconName(it)) // no icon anywhere: fail naming the one that is missing
}

// d1GoldIconName picks the gold pile icon by thirds of the spawned range: 5*dlvl + rand(10*dlvl), +1/8 in hell,
// dlvl 1..16 gives 5..268.
func d1GoldIconName(value int) string {
	const lo, hi = 5, 268
	switch {
	case value < lo+(hi-lo)/3:
		return "item.gold_small"
	case value < lo+2*(hi-lo)/3:
		return "item.gold_medium"
	}
	return "item.gold_large"
}

func d1ItemIconName(it *d1.Item) string {
	if it.Type == d1.ITypeGold {
		return d1GoldIconName(it.Value)
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
	return d1ITypeFallback[it.Type]
}

// d1SlotName is the slot an item goes in; dualWield (the Bard) puts a one-handed weapon in either free hand,
// which SlotNameOneHandedWeapon stands for the way SlotNameRing stands for either ring finger.
func d1SlotName(it *d1.Item, dualWield bool) defs.SlotName {
	switch it.Loc {
	case "ILOC_ONEHAND":
		if it.Type == d1.ITypeShield {
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

// NewD1Item wraps a rolled D1 item into a game item; the D1 fields stay the source of truth.
func (g *Factory) NewD1Item(it *d1.Item) *Item {
	item := NewItem(g.State.GetPlayer, it.Name, g.d1ItemIcon(it))
	item.d1, item.d1Data = it, g.Generator.D
	item.SetInternalName(it.Name)
	item.SetBaseCost(it.IValue)
	if hooks := d1ItemHooks(it); len(hooks) > 0 {
		item.SetItemHooks(hooks)
	}
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
				Damage:        util.NewInterval(it.MinDam, it.MaxDam),
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
		item.SetThrowComponent(&ThrowStats{ // house rule, D1 has no throwing
			Damage:        util.NewInterval(0, it.AC),
			ChanceToBreak: 0.25,
			HitZone:       OneTargetTile(g.State.GetMap, nil),
		})
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
		default:
			if d1.IsOil(it.MiscID) { // Hellfire's oils: pick an item out of the pack and pour it on
				g.d1OilAction(item, it.MiscID, it.Name)
			}
			g.d1RuneAction(item, it)
		}
	}
	if it.Type == d1.ITypeStaff && it.Spell > 0 && it.MaxCharges > 0 {
		g.d1SpellItemAction(item)
	}
	if it.BaseIdx >= 0 && it.BaseIdx < len(g.Generator.D.Items) {
		g.applyItemRecord(item, g.Generator.D.Items[it.BaseIdx])
	}
	item.SetRarity(ItemRarity(it.Magical))
	item.RefreshD1()
	return item
}

func (i *Item) IsGold() bool { return i.d1 != nil && i.d1.Type == d1.ITypeGold }

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
	if it.MaxCharges > 0 && i.HasAction() {
		i.actionCharges = it.Charges
	}
	i.forcedTooltip = util.ToLabelText(append([]string{""}, i.d1TooltipLines(nil)...))
}

// D1TooltipLinesComparedTo is GetTooltipLines with Damage, Armor and property lines colored by cmp (d1.Item.Compare):
// green where this item is better, red where it is worse.
func (i *Item) D1TooltipLinesComparedTo(cmp map[string]int, parseColorCodedText func(string) []util.ColoredTextPart) []util.LabelText {
	i.RefreshD1()
	lines := util.ToLabelText(append([]string{i.Name(), ""}, i.d1TooltipLines(cmp)...))
	lines[0].TextColor = defs.OffWhite
	return parseColorCodes(lines, parseColorCodedText)
}

func (i *Item) d1TooltipLines(cmp map[string]int) []string {
	it := i.d1
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
	if it.Spell > 0 && it.Spell < len(d1.SpellDescriptions) {
		lines = append(lines, d1.SpellDescriptions[it.Spell])
		// what the spell does for this hero, right under what it is (a staff's own melee damage follows below)
		if player := i.getPlayer(); player != nil && player.hero != nil {
			h, sh := player.hero, player.HeroSheet()
			if it.Spell == d1.SplHeal {
				lo, hi := d1.HealRange64(h.Class, h.Level, h.SpellLevel(sh, it.Spell))
				add("Spell heals %d-%d life", lo>>6, hi>>6)
			}
			if t := d1.SpellDamageText(it.Spell, sh.Mag, h.Level, h.SpellLevel(sh, it.Spell)); t != "" {
				add("Spell: %s", t)
			}
		}
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
		add("%s", colored(cmp[d1.KeyDamage], fmt.Sprintf("Damage: %d-%d", it.MinDam, it.MaxDam)))
	}
	if it.AC > 0 {
		add("%s", colored(cmp[d1.KeyArmor], fmt.Sprintf("Armor: %d", it.AC)))
	}
	// what the weapon's kind does on its own, before any affix: PlrHitMonst's sword/mace rule
	switch it.Type {
	case d1.ITypeSword:
		add("[:positive]+50%% damage vs. animals[:white], [:negative]-50%% vs. the undead[:white]")
	case d1.ITypeMace:
		add("[:positive]+50%% damage vs. the undead[:white], [:negative]-50%% vs. animals[:white]")
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
		for _, l := range it.PropertyLines(cmp) {
			if l.Cmp == 0 { // affixes stand out from the base stats; a comparison's green/red wins over the blue
				lines = append(lines, "[:magic]"+l.Text+"[:white]")
				continue
			}
			lines = append(lines, colored(l.Cmp, l.Text))
		}
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
	return lines
}

// NoteUnequipped marks an equipped item that equipping another would take off without a replacement.
const NoteUnequipped = "Would be unequipped"

// EquippedComparison is an equipped item equipping another item would displace, with its values compared to the other
// item's (Cmp, see d1.Item.Compare) and an optional note.
type EquippedComparison struct {
	Item *Item
	Cmp  map[string]int
	Note string
}

// D1CompareForEquipping lists the equipped items candidate (a shop or pack item) would replace (left ring first, weapon before shield) and how
// candidate's values compare to them: per key better or worse than all of them, else 0. Empty if nothing is displaced.
func (e *Equipment) D1CompareForEquipping(candidate *Item) ([]EquippedComparison, map[string]int) {
	if candidate.d1 == nil {
		return nil, nil
	}
	var result []EquippedComparison
	var candidateCmps []map[string]int
	compare := func(worn *Item) {
		if worn == nil || worn.d1 == nil {
			return
		}
		wornKnown := !(worn.d1.Magical > 0 && !worn.d1.Identified)
		known := wornKnown && !(candidate.d1.Magical > 0 && !candidate.d1.Identified)
		c := EquippedComparison{Item: worn, Cmp: worn.d1.Compare(candidate.d1, known)}
		if !wornKnown { // an unidentified candidate says so in its own tooltip
			c.Note = "Affixes unknown (not identified)"
		}
		result = append(result, c)
		candidateCmps = append(candidateCmps, candidate.d1.Compare(worn.d1, known))
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
		if shield := e.GetBySlot(defs.SlotNameOffHand); shield != nil && shield.d1 != nil {
			lost := shield.d1.Compare(nil, !(shield.d1.Magical > 0 && !shield.d1.Identified))
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
func (g *Factory) applyItemRecord(item *Item, b d1.ItemData) {
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
		light := d1HeroTorch
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
		lvl := max(b.DLvl, 1)
		switch m {
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
