package common

import (
	"fmt"
	"rpg/d1"
	"rpg/stats"
	"rpg/util"
)

// D1 townspeople (decision 10). A vendor type is the NPC's InternalName in maps.ldtk and its gossip key in d1.Data.Gossip.
const (
	VendorTypeCain    VendorType = "cain"
	VendorTypeWirt    VendorType = "wirt"
	VendorTypeOgden   VendorType = "ogden"   // no store in single player, only talk
	VendorTypeDeadGuy VendorType = "deadguy" // the wounded townsman: only his last words
	VendorTypeDrunk   VendorType = "farnham" // only talk (S_StartDrunk)
	VendorTypeBarmaid VendorType = "gillian" // only talk (S_StartBarmaid)
)

// WirtLookCost is what Wirt charges to show his item.
const WirtLookCost = 50

// D1VendorBuys ports SmithSellOk/WitchSellOk for the vendor's type.
func D1VendorBuys(vendor *Actor, item *Item) bool {
	if item.d1 == nil || item.d1Data == nil {
		return false
	}
	switch vendor.GetVendorType() {
	case VendorTypeWeapons:
		return item.d1Data.SmithBuys(item.d1)
	case VendorTypeWands:
		return item.d1Data.WitchBuys(item.d1)
	}
	return false
}

// D1Shopper is what Hellfire's stores look at of the player (SpawnOnePremium, SpawnBoy, HealerItemOk).
func D1Shopper(player *Actor) *d1.Shopper {
	h, s := player.hero, player.HeroSheet()
	if h == nil {
		return nil
	}
	who := &d1.Shopper{Class: h.Class, Str: s.Str, Mag: s.Mag, Dex: s.Dex, BaseStr: h.BaseStr, BaseMag: h.BaseMag,
		BaseDex: h.BaseDex, BaseVit: h.BaseVit}
	for _, it := range d1HeroItems(player) {
		if it.d1 != nil {
			who.Owned = append(who.Owned, it.d1)
		}
	}
	return who
}

func D1SellPrice(item *Item) int { return d1.SellPrice(item.d1) }

// PayGold takes gold from the player; false (with a message) if there isn't enough.
func PayGold(ctx ActionContext, player *Actor, cost int) bool {
	if player.GetStats().GetAttribute(stats.Gold) < cost {
		ctx.Msg("You don't have enough gold.")
		return false
	}
	player.GetStats().DecrementBy(stats.Gold, cost)
	return true
}

// d1ServiceOption lists the player's items a service applies to, with prices, and performs it on selection. Like the
// sell menu, the list reopens after each purchase until nothing is left for the service.
func d1ServiceOption(ctx ActionContext, label string, applies func(*d1.Item) bool, cost func(*d1.Item) int, perform func(*d1.Item)) util.MenuItem {
	var open func(reopen bool)
	open = func(reopen bool) {
		player := ctx.GameState.GetPlayer()
		var items []*Item
		for _, item := range d1HeroItems(player) {
			if item.d1 != nil && applies(item.d1) {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			if !reopen {
				ctx.Msg("You have nothing for that.")
			}
			return
		}
		ctx.UserInterface().OpenServiceMenu(items, func(it *Item) int { return cost(it.d1) }, func(it *Item) {
			price := cost(it.d1)
			if PayGold(ctx, player, price) {
				perform(it.d1)
				it.RefreshD1()
				SyncHero(player)
				ctx.Msg(fmt.Sprintf("%s: done (%d gold)", it.Name(), price))
			}
			open(true)
		})
	}
	return util.MenuItem{MainText: label, Action: func() { open(false) }}
}

// d1VendorOptions ports each NPC's S_Start* menu (stores.cpp) with its exact D1 labels and structure: Griswold
// splits his stock into basic and premium buy lists (S_StartSmith), Adria has one buy list plus recharging
// (S_StartWitch), Pepin's free heal is its own choice rather than a side effect of talking (S_StartHealer, base
// game), Cain only identifies (S_StartStory), and Wirt only offers his one item for a look fee (S_StartBoy).
func d1VendorOptions(ctx ActionContext, vendor *Actor) []util.MenuItem {
	gui := ctx.UserInterface()
	data := ctx.GameState.factory.D1().D
	sell := util.MenuItem{MainText: "Sell items", Action: func() { gui.BeginPlayerSellingTo(vendor) }}
	switch vendor.GetVendorType() {
	case VendorTypeWeapons:
		basic := util.MenuItem{MainText: "Buy basic items", Action: func() {
			gui.OpenShopMenuFiltered(vendor, func(item *Item) bool { return !item.IsPremiumStock() })
		}}
		premium := util.MenuItem{MainText: "Buy premium items", Action: func() {
			gui.OpenShopMenuFiltered(vendor, func(item *Item) bool { return item.IsPremiumStock() })
		}}
		return []util.MenuItem{basic, premium, sell, d1ServiceOption(ctx, "Repair items", d1.CanRepair, d1.RepairCost,
			func(it *d1.Item) { it.Durability = it.MaxDur })}
	case VendorTypeWands:
		buy := util.MenuItem{MainText: "Buy items", Action: func() { gui.OpenShopMenu(vendor) }}
		return []util.MenuItem{buy, sell, d1ServiceOption(ctx, "Recharge staves", data.CanRecharge, data.RechargeCost,
			func(it *d1.Item) { it.Charges = it.MaxCharges })}
	case VendorTypeGeneral:
		heal := func() {
			player := ctx.GameState.GetPlayer()
			if hp := player.GetStats().GetResource(stats.Health); !hp.IsAtMax() {
				ctx.GameState.factory.playD1Sound("IS_CAST8")
			}
			player.HealCompletely()
			ctx.Msg("Pepin tends to your wounds.")
		}
		buy := util.MenuItem{MainText: "Buy items", Action: func() { gui.OpenShopMenu(vendor) }}
		if data.Hellfire { // S_StartHealer: Hellfire's Pepin heals as you come in
			heal()
			return []util.MenuItem{buy}
		}
		return []util.MenuItem{{MainText: "Receive healing", Action: heal}, buy}
	case VendorTypeCain:
		// D1 charges a flat 100 gold to identify (AddStoreHoldId in stores.cpp), regardless of item value
		return []util.MenuItem{d1ServiceOption(ctx, "Identify an item", func(it *d1.Item) bool { return it.Magical != 0 && !it.Identified },
			func(*d1.Item) int { return 100 }, func(it *d1.Item) { it.Identified = true })}
	case VendorTypeWirt: // the look fee is paid once per item: looking again is free until Wirt has a new one
		if vendor.GetInventory().IsEmpty() {
			return nil
		}
		flags := ctx.GameState.GetFlags()
		label := fmt.Sprintf("I have something for sale, but it will cost %d gold just to take a look", WirtLookCost)
		if flags.IsSet(FlagWirtItemSeen) {
			label = "What have you got?"
		}
		return []util.MenuItem{{MainText: label, Action: func() {
			if flags.IsSet(FlagWirtItemSeen) || PayGold(ctx, ctx.GameState.GetPlayer(), WirtLookCost) {
				flags.SetTrue(FlagWirtItemSeen)
				gui.OpenShopMenu(vendor)
			}
		}}}
	}
	return nil
}

func (a *Actor) equippedD1() []*d1.Item {
	var items []*d1.Item
	for _, it := range a.GetEquipment().AllItems() {
		if it.d1 != nil {
			items = append(items, it.d1)
		}
	}
	return items
}

// d1Broke reports an item that just broke and recalculates the sheet.
func d1Broke(ctx ActionContext, owner *Actor, broke *d1.Item) {
	if broke == nil {
		return
	}
	name := broke.Name
	if broke.Identified {
		name = broke.IName
	}
	ctx.Msg(fmt.Sprintf("Your %s broke! Griswold can repair it.", name))
	SyncHero(owner)
}

// WeaponAboutToBreak returns the equipped weapon for attackType if it is already at zero durability, i.e.
// swinging it now destroys it. Used to warn the player before such an attack.
func WeaponAboutToBreak(actor *Actor, attackType AttackType) *Item {
	weapon := actor.GetEquipment().GetWeapon(attackType)
	if weapon == nil || weapon.d1 == nil || weapon.d1.MaxDur == 0 || weapon.d1.Durability != 0 {
		return nil
	}
	return weapon
}

// destroyBrokenWeapon removes a weapon that was used at zero durability from the owner entirely.
func destroyBrokenWeapon(ctx ActionContext, owner *Actor, weapon *Item) {
	name := weapon.Name()
	owner.GetEquipment().UnEquip(weapon)
	owner.GetInventory().Remove(weapon)
	ctx.Msg(fmt.Sprintf("Your %s falls apart!", name))
	SyncHero(owner)
}

// FlagWirtItemSeen is set once the look fee for Wirt's current item is paid; a new item clears it (saved with the flags).
const FlagWirtItemSeen = "wirt_item_seen"
