package common

import (
	"fmt"
	"rpg/d1"
	"rpg/stats"
	"rpg/util"
)

// D1 town vendors (decision 10). The H&H vendor types keep their ids; their meaning is D1's.
const (
	VendorTypeCain VendorType = "store_cain"
	VendorTypeWirt VendorType = "store_wirt"
)

// WirtLookCost is what Wirt charges to show his item.
const WirtLookCost = 50

// D1VendorBuys ports SmithSellOk/WitchSellOk for the vendor's type.
func D1VendorBuys(vendor *Actor, item *Item) bool {
	if item.d1 == nil {
		return false
	}
	switch vendor.GetVendorType() {
	case VendorTypeWeapons:
		return d1.SmithBuys(item.d1)
	case VendorTypeWands:
		return d1.WitchBuys(item.d1)
	}
	return false
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

// d1ServiceOption lists the player's items a service applies to, with prices, and performs it on selection.
func d1ServiceOption(ctx ActionContext, label string, applies func(*d1.Item) bool, cost func(*d1.Item) int, perform func(*d1.Item)) util.MenuItem {
	return util.MenuItem{MainText: label, Action: func() {
		player := ctx.GameState.GetPlayer()
		var items []util.MenuItem
		for _, item := range player.GetInventory().Items() {
			if item.d1 == nil || !applies(item.d1) {
				continue
			}
			it := item
			price := cost(it.d1)
			items = append(items, util.MenuItem{MainText: fmt.Sprintf("%s (%d gold)", it.Name(), price), Action: func() {
				if !PayGold(ctx, player, price) {
					return
				}
				perform(it.d1)
				it.RefreshD1()
				SyncHero(player)
				ctx.Msg(fmt.Sprintf("%s: done (%d gold)", it.Name(), price))
			}})
		}
		if len(items) == 0 {
			ctx.Msg("You have nothing for that.")
			return
		}
		ctx.UserInterface().OpenMenu(items)
	}}
}

func d1VendorOptions(ctx ActionContext, vendor *Actor) []util.MenuItem {
	gui := ctx.UserInterface()
	data := ctx.GameState.factory.D1().D
	buy := util.MenuItem{MainText: "BUY", Action: func() { gui.OpenShopMenu(vendor) }}
	sell := util.MenuItem{MainText: "SELL", Action: func() { gui.BeginPlayerSellingTo(vendor) }}
	switch vendor.GetVendorType() {
	case VendorTypeWeapons:
		return []util.MenuItem{buy, sell, d1ServiceOption(ctx, "REPAIR", d1.CanRepair, d1.RepairCost,
			func(it *d1.Item) { it.Durability = it.MaxDur })}
	case VendorTypeWands:
		return []util.MenuItem{buy, sell, d1ServiceOption(ctx, "RECHARGE", d1.CanRecharge, data.RechargeCost,
			func(it *d1.Item) { it.Charges = it.MaxCharges })}
	case VendorTypeGeneral:
		return []util.MenuItem{buy}
	case VendorTypeCain:
		// decision 9: Cain identifies for free (D1 charges 100 gold)
		return []util.MenuItem{d1ServiceOption(ctx, "IDENTIFY", func(it *d1.Item) bool { return it.Magical != 0 && !it.Identified },
			func(*d1.Item) int { return 0 }, func(it *d1.Item) { it.Identified = true })}
	case VendorTypeWirt:
		return []util.MenuItem{{MainText: fmt.Sprintf("LOOK (%d gold)", WirtLookCost), Action: func() {
			if vendor.GetInventory().IsEmpty() {
				ctx.Msg("Wirt has nothing for sale.")
				return
			}
			if PayGold(ctx, ctx.GameState.GetPlayer(), WirtLookCost) {
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
