package common

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/stats"
	"rpg/util"
	"slices"
	"strings"
)

// Menu contents and shop transactions shared by the tile UI (gui) and the ASCII UI (console).

// QuickSlotEntry is anything the belt or the skill bar can hold and fire.
type QuickSlotEntry interface {
	Name() string
	InventoryIcon() int32
	Counter() int
	Use(act PlayerActions)
}

// BeltSize is how many quick slots the belt has, on keys 1 to BeltSize.
const BeltSize = 8

// SkillBar is the sidebar's skill bar: the memorized spells and class skill (*TargetedAction, *UntargetedAction), then
// the staves in the pack (*Item), in the order the player dragged them into (Hero.SkillOrder); new ones go last.
func SkillBar(player *Actor) []QuickSlotEntry {
	var bar []QuickSlotEntry
	for _, skill := range player.GetTargetedSkills() {
		bar = append(bar, skill)
	}
	for _, skill := range player.GetUntargetedSkills() {
		bar = append(bar, skill)
	}
	for _, item := range player.GetInventory().Items() { // equippable items with a spell (staves), while carried
		if item.IsEquippable() && item.HasAction() {
			bar = append(bar, item)
		}
	}
	if h := player.Hero(); h != nil {
		rank := func(e QuickSlotEntry) int {
			if i := slices.Index(h.SkillOrder, skillBarKey(e)); i >= 0 {
				return i
			}
			return len(h.SkillOrder)
		}
		slices.SortStableFunc(bar, func(a, b QuickSlotEntry) int { return rank(a) - rank(b) })
	}
	return bar
}

// skillBarKey is what Hero.SkillOrder remembers an entry by: its internal name (a staff's is its base type's).
func skillBarKey(e QuickSlotEntry) string {
	return e.(interface{ GetInternalName() string }).GetInternalName()
}

// MoveSkill drags the skill bar entry at from to slot to and remembers the new order.
func MoveSkill(player *Actor, from, to int) {
	bar := SkillBar(player)
	if player.Hero() == nil || from < 0 || from >= len(bar) || to < 0 {
		return
	}
	to = min(to, len(bar)-1)
	moved := bar[from]
	bar = slices.Insert(slices.Delete(bar, from, from+1), to, moved)
	order := make([]string, len(bar))
	for i, e := range bar {
		order[i] = skillBarKey(e)
	}
	player.Hero().SkillOrder = order
}

func (a *UntargetedAction) Use(act PlayerActions) {
	act.ActivateUntargetedAction(a, EffectSource(a.GetInternalName()))
}

func (t *TargetedAction) Use(act PlayerActions) { act.BeginActionTargeting(t) }

// Use does the obvious thing with a carried item: a staff (or anything else with a spell) casts it, other gear is
// equipped or taken off, potions and scrolls are used.
func (i *Item) Use(act PlayerActions) {
	actions := act.GetContextActionsForItem(i)
	if !i.HasAction() {
		actions = ItemActions(act, i.getPlayer(), i, func() {})
	}
	if len(actions) > 0 {
		actions[0].Action()
	}
}

// Use uses the stack's first item; an emptied stack does nothing.
func (s *InventoryStack) Use(act PlayerActions) {
	if len(*s) > 0 {
		(*s)[0].Use(act)
	}
}

// BeltEntry is what belt slot i holds right now: nil when it is empty, its stack is used up, or its skill is no
// longer known (a staff's spell once the staff is gone).
func BeltEntry(player *Actor, i int) QuickSlotEntry {
	switch e := player.belt[i].(type) {
	case *InventoryStack:
		if len(*e) > 0 {
			return e
		}
	case *TargetedAction:
		if slices.Contains(player.GetTargetedSkills(), e) {
			return e
		}
	case *UntargetedAction:
		if slices.Contains(player.GetUntargetedSkills(), e) {
			return e
		}
	}
	return nil
}

// SetBeltEntry puts e (nil to clear) into belt slot i; e leaves any other slot, as a thing is on the belt once.
func SetBeltEntry(player *Actor, i int, e QuickSlotEntry) {
	for j, other := range player.belt {
		if e != nil && other == e {
			player.belt[j] = nil
		}
	}
	player.belt[i] = e
}

// SwapBeltEntries swaps two belt slots.
func SwapBeltEntries(player *Actor, i, j int) {
	player.belt[i], player.belt[j] = player.belt[j], player.belt[i]
}

// SavedBeltSlot is a belt slot in a save: Item is 1 + the index of its stack's first item in the saved item list,
// Skill a skill's internal name; the zero value is an empty slot.
type SavedBeltSlot struct {
	Item  int    `json:",omitempty"`
	Skill string `json:",omitempty"`
}

// SaveBelt writes the belt against items, the order the save lists the hero's items in.
func SaveBelt(player *Actor, items []*Item) (saved [BeltSize]SavedBeltSlot) {
	for i := range player.belt {
		switch e := BeltEntry(player, i).(type) {
		case *InventoryStack:
			saved[i].Item = slices.Index(items, (*e)[0]) + 1
		case interface{ GetInternalName() string }:
			saved[i].Skill = e.GetInternalName()
		}
	}
	return saved
}

// LoadBelt restores a belt SaveBelt wrote; items must be in the same order and carried, the skills refreshed.
func LoadBelt(player *Actor, saved [BeltSize]SavedBeltSlot, items []*Item) {
	player.belt = [BeltSize]QuickSlotEntry{}
	for i, slot := range saved {
		if slot.Item > 0 && slot.Item <= len(items) {
			if stack := player.GetInventory().StackOf(items[slot.Item-1]); stack != nil {
				player.belt[i] = stack
			}
		}
		for _, skill := range SkillBar(player) {
			if _, isItem := skill.(*Item); !isItem && slot.Skill != "" && skillBarKey(skill) == slot.Skill {
				player.belt[i] = skill
			}
		}
	}
}

// SpellHotkeys are the spell menu's keys, in skill bar order: the number row first, then the letters.
const SpellHotkeys = "1234567890abcdefghijklmnopqrstuvwxyz"

// SpellHotkey is the spell menu key of skill bar slot i (scrolls continue after the bar), 0 past the last.
func SpellHotkey(i int) rune {
	if i < 0 || i >= len(SpellHotkeys) {
		return 0
	}
	return rune(SpellHotkeys[i])
}

// SpellMenuItems lists the skill bar, then D1's spell list extras: scrolls, and staves in hand. A skill's key is
// its skill bar slot's, so the bar's hints match; the extras take the keys after the bar. Lines are white for
// skills, the item's color for scrolls and staves, and red when they can't be cast now (no mana, no charges).
func SpellMenuItems(state *State, act PlayerActions) []util.MenuItem {
	player := state.GetPlayer()
	bar := SkillBar(player)
	var items []util.MenuItem
	nextExtraKey := len(bar)
	add := func(slot int, label string, icon int32, tint color.RGBA, tooltip []util.LabelText, action func()) {
		key := SpellHotkey(slot)
		prefix := "   "
		if key != 0 {
			prefix = fmt.Sprintf("%c) ", key)
		}
		items = append(items, util.MenuItem{MainText: prefix + label, MainTextColor: tint, CharIcon: icon, TooltipText: tooltip, Hotkey: key, Action: action})
	}
	d1Label := func(it *Item) string {
		d := it.D1()
		if d.Type == d1.ITypeStaff {
			return fmt.Sprintf("%s (Staff, %d charges)", state.D1SpellName(d.Spell), d.Charges)
		}
		return fmt.Sprintf("%s (Scroll)", state.D1SpellName(d.Spell))
	}
	itemTint := func(it *Item) color.RGBA {
		if !it.HasChargesLeft() {
			return defs.Red
		}
		return it.TintColor()
	}
	itemTooltip := func(it *Item) []util.LabelText {
		return []util.LabelText{util.LabelTextFromStringWithColor(it.Name(), it.TintColor())}
	}
	seen := map[*Item]bool{}
	seenLabel := map[string]bool{}
	for slot, entry := range bar {
		switch e := entry.(type) {
		case *Item:
			if e.D1() == nil || e.D1().Spell == 0 {
				continue
			}
			seen[e] = true // every slot is listed, so the bar's hints all work; only the extras skip repeats
			add(slot, d1Label(e), e.InventoryIcon(), itemTint(e), itemTooltip(e), func() { e.Use(act) })
		case interface {
			InventoryIcon() int32
			GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText
		}:
			tint := defs.White
			if state.D1LacksMana(player, entry.Name()) {
				tint = defs.Red
			}
			add(slot, entry.Name(), e.InventoryIcon(), tint, e.GetTooltipLines(nil), func() { entry.Use(act) })
		}
	}
	for _, it := range append(player.GetInventory().Items(), player.GetEquipment().AllItems()...) {
		d := it.D1()
		if seen[it] || d == nil || d.Spell == 0 || d.MiscID != "IMISC_SCROLL" && d.MiscID != "IMISC_SCROLLT" && d.Type != d1.ITypeStaff {
			continue
		}
		label := d1Label(it)
		if seenLabel[label+it.Name()] { // a stack of the same scroll is one line
			continue
		}
		seenLabel[label+it.Name()] = true
		for _, action := range act.GetContextActionsForItem(it) {
			add(nextExtraKey, label, it.InventoryIcon(), itemTint(it), itemTooltip(it), action.Action)
			nextExtraKey++
		}
	}
	return items
}

// LevelUpMenuItems is the stat point header and one entry per stat with its base value, its value with items when
// that differs, and the class maximum. Picking a stat spends a point, then calls reopen while points are left, done otherwise.
func LevelUpMenuItems(player *Actor, reopen, done func()) []util.MenuItem {
	var items []util.MenuItem
	h, sheet := player.Hero(), player.HeroSheet()
	if h != nil {
		items = append(items, util.MenuItem{MainText: fmt.Sprintf("%d stat points to spend", h.StatPts), Action: reopen}, util.MenuItem{})
	}
	for _, choice := range []struct {
		label string
		stat  stats.Name
		attr  int
	}{{"Strength", stats.Strength, d1.AttrStr}, {"Magic", stats.Magic, d1.AttrMag}, {"Dexterity", stats.Dexterity, d1.AttrDex}, {"Vitality", stats.Vitality, d1.AttrVit}} {
		stat := choice.stat
		label := choice.label
		if h != nil {
			base := [4]int{h.BaseStr, h.BaseMag, h.BaseDex, h.BaseVit}[choice.attr]
			total := [4]int{sheet.Str, sheet.Mag, sheet.Dex, sheet.Vit}[choice.attr]
			label = fmt.Sprintf("%s %d", label, base)
			if total != base {
				label += fmt.Sprintf(" (%d)", total)
			}
			label += fmt.Sprintf(" / %d", d1.MaxStats[h.Class][choice.attr])
		}
		items = append(items, util.MenuItem{MainText: label, Action: func() {
			player.LevelUp(stat)
			if player.CanLevelUp() {
				reopen()
			} else {
				done()
			}
		}})
	}
	return items
}

// BuyItem moves item from seller to player if the player can pay, and returns the message for the player.
func BuyItem(player, seller *Actor, item *Item) string {
	cost := item.BuyPrice()
	if cost > player.GetStats().GetAttribute(stats.Gold) {
		return "You don't have enough gold"
	}
	player.GetStats().DecrementBy(stats.Gold, cost)
	seller.GetStats().IncrementBy(stats.Gold, cost)
	seller.GetInventory().Remove(item)
	player.GetInventory().Add(item)
	return fmt.Sprintf("You bought [:darkText]%s[:white] for [:darkText]%d[:white] gold", item.Name(), cost)
}

// SellableStacks are the pack's stacks vendor buys: D1 stores buy only their kind of goods, and only from the pack.
func SellableStacks(vendor, player *Actor) []InventoryStack {
	var result []InventoryStack
	for _, stack := range player.GetInventory().StackedItems() {
		if D1VendorBuys(vendor, stack[0]) && !player.GetEquipment().IsEquipped(stack[0]) {
			result = append(result, stack)
		}
	}
	return result
}

// SellItem sells item from the player's pack and returns the message for the player.
func SellItem(player *Actor, item *Item) string {
	price := D1SellPrice(item)
	player.GetInventory().Remove(item)
	player.GetStats().IncrementBy(stats.Gold, price)
	return fmt.Sprintf("You sold [:darkText]%s[:white] for [:darkText]%d[:white] gold", item.Name(), price)
}

// ShopItemName names a stack the way config.rec's ItemNamesInShop asks for.
func ShopItemName(scheme ItemNamingScheme, stack InventoryStack) string {
	switch scheme {
	case ItemNamingSchemeEnchantments:
		return stack.PlusName()
	case ItemNamingSchemeBandStyle:
		return stack.BandStyleName()
	}
	return stack.Name()
}

// configToggle is a menu item that flips a boolean setting, saves config.rec and calls reopen.
func configToggle(cfg *Configuration, name string, value *bool, reopen func()) util.MenuItem {
	state := "Off"
	if *value {
		state = "On"
	}
	return util.MenuItem{MainText: fmt.Sprintf("%s: %s", name, state), Action: func() {
		*value = !*value
		cfg.SaveToRecFile(ConfigFile)
		reopen()
	}}
}

// AutoExploreSettings are the pause menu's auto explore toggles.
func AutoExploreSettings(cfg *Configuration, reopen func()) []util.MenuItem {
	return []util.MenuItem{
		configToggle(cfg, "Open Containers", &cfg.AutoExploreOpenContainers, reopen),
		configToggle(cfg, "Pickup loot", &cfg.AutoPickup, reopen),
	}
}

// RandomHeroName picks a name for a new hero, or "" when the name lists are missing.
// Name lists: michael-aloys/little-namegenerator (CC BY 4.0, Jonas Förster & Michael A. Hedderich).
func RandomHeroName(female bool) string {
	file := "assets/names-male.txt"
	if female {
		file = "assets/names-female.txt"
	}
	data, err := files.ReadFile(file)
	if err != nil {
		return ""
	}
	names := strings.Split(strings.TrimSpace(string(data)), "\n")
	return names[rand.IntN(len(names))]
}

// ItemActions is what using an inventory item can do: equip or unequip it, or its context actions (drink, read...).
// A staff's spell is cast from the skill bar instead. A UI runs a single one right away and offers a menu for several.
func ItemActions(act PlayerActions, player *Actor, item *Item, afterEquip func()) []util.MenuItem {
	if !item.IsEquippable() {
		return act.GetContextActionsForItem(item)
	}
	wasEquipped := player.GetEquipment().IsEquipped(item)
	label := "Equip"
	if wasEquipped {
		label = "Unequip"
	}
	equip := util.MenuItem{MainText: label, Action: func() {
		if wasEquipped {
			act.UnequipItem(item)
		} else {
			act.EquipItem(item)
		}
		afterEquip()
	}}
	return []util.MenuItem{equip}
}
