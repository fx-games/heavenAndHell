package common

import (
	"fmt"
	"math/rand/v2"
	"rpg/d1"
	"rpg/defs"
	"rpg/files"
	"rpg/stats"
	"rpg/util"
	"strings"
)

// Menu contents and shop transactions shared by the tile UI (gui) and the ASCII UI (console).

// SpellMenuItems lists the memorized D1 spells; picking one casts it (targeted spells start targeting first).
// D1's spell list also offers the spells on scrolls and staves in the pack or in hand (one entry each).
func SpellMenuItems(state *State, act PlayerActions) []util.MenuItem {
	player := state.GetPlayer()
	var items []util.MenuItem
	for _, skill := range player.GetTargetedSkills() {
		s := skill
		items = append(items, util.MenuItem{MainText: s.Name(), Action: func() { act.BeginActionTargeting(s) }})
	}
	for _, skill := range player.GetUntargetedSkills() {
		s := skill
		items = append(items, util.MenuItem{MainText: s.Name(), Action: func() {
			act.ActivateUntargetedAction(s, EffectSource(s.GetInternalName()))
		}})
	}
	seen := map[string]bool{}
	for _, it := range append(player.GetInventory().Items(), player.GetEquipment().AllItems()...) {
		d := it.D1()
		if d == nil || d.Spell == 0 || d.MiscID != "IMISC_SCROLL" && d.MiscID != "IMISC_SCROLLT" && d.Type != d1.ITypeStaff {
			continue
		}
		label := fmt.Sprintf("%s (%s)", state.D1SpellName(d.Spell), it.Name())
		if d.Type == d1.ITypeStaff {
			label = fmt.Sprintf("%s (%s, %d charges)", state.D1SpellName(d.Spell), it.Name(), d.Charges)
		}
		if seen[label] {
			continue
		}
		seen[label] = true
		for _, action := range act.GetContextActionsForItem(it) {
			items = append(items, util.MenuItem{MainText: label, Action: action.Action})
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

// HellfireSetting is the start menu's Hellfire toggle.
func HellfireSetting(cfg *Configuration, reopen func()) util.MenuItem {
	return configToggle(cfg, "Hellfire content", &cfg.Hellfire, reopen)
}

// PlayableClasses drops Hellfire's Monk, Bard and Barbarian (classes.rec's last three) when Hellfire is off.
func PlayableClasses(cfg *Configuration, classes []defs.LoadoutDefinition) []defs.LoadoutDefinition {
	if cfg.Hellfire {
		return classes
	}
	return classes[:d1.Monk]
}

// GameplaySettings are the pause menu's gameplay toggles.
func GameplaySettings(cfg *Configuration, reopen func()) []util.MenuItem {
	return []util.MenuItem{configToggle(cfg, "Warning: Weapon breaking", &cfg.WeaponBreakWarning, reopen)}
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

// ItemActions is what using an inventory item can do: equip or unequip it (a staff can also cast its spell), or its
// context actions (drink, read...). A UI runs a single one right away and offers a menu for several.
func ItemActions(act PlayerActions, player *Actor, item *Item, afterEquip func()) []util.MenuItem {
	contextActions := act.GetContextActionsForItem(item)
	if !item.IsEquippable() {
		return contextActions
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
	for i := range contextActions {
		contextActions[i].MainText = "Cast " + contextActions[i].MainText
	}
	return append([]util.MenuItem{equip}, contextActions...)
}
