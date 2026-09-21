package common

import (
	"fmt"

	"github.com/expr-lang/expr"

	"rpg/d1"
	"rpg/defs"
	"rpg/stats"
	"rpg/util"
	"sort"
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

// D1VendorBuys is whether vendor takes item off the hero: its vendor records' buys rule (SmithSellOk,
// WitchSellOk, or anything).
func D1VendorBuys(vendor *Actor, item *Item) bool {
	if item.d1 == nil || item.d1Data == nil {
		return false
	}
	for _, v := range item.d1Data.VendorsOf(vendor.GetInternalName()) {
		switch v.Buys {
		case "smith":
			return item.d1Data.SmithBuys(item.d1)
		case "witch":
			return item.d1Data.WitchBuys(item.d1)
		case "hellfire_smith":
			return item.d1Data.HellfireSmithBuys(item.d1)
		case "hellfire_witch":
			return item.d1Data.HellfireWitchBuys(item.d1)
		case "any":
			return true
		}
	}
	return false
}

// D1Shopper is what stores with Hellfire's options look at of the player (SpawnOnePremium, SpawnBoy, HealerItemOk).
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
// showAfter, when set, opens the item's description on top of the reopened list (or the NPC's menu, once nothing is
// left for the service), so closing it goes back there.
func d1ServiceOption(ctx ActionContext, label string, applies func(*d1.Item) bool, cost func(*d1.Item) int, perform func(*d1.Item), showAfter ...bool) util.MenuItem {
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
			done := PayGold(ctx, player, price)
			if done {
				perform(it.d1)
				it.RefreshD1()
				SyncHero(player)
				ctx.Msg(fmt.Sprintf("%s: done (%d gold)", it.Name(), price))
			}
			open(true)
			if done && len(showAfter) > 0 && showAfter[0] {
				ctx.UserInterface().ShowItemInfo(it)
			}
		})
	}
	return util.MenuItem{MainText: label, Action: func() { open(false) }}
}

// d1VendorOptions is the town NPC's menu: the services of each of its vendor records whose condition holds, in
// stores.rec order. Every service is implemented here once; a mod only picks which ones an NPC offers. D1's
// labels are kept (S_StartSmith, S_StartWitch, S_StartHealer, S_StartStory, S_StartBoy).
func d1VendorOptions(ctx ActionContext, vendor *Actor) []util.MenuItem {
	var items []util.MenuItem
	f := ctx.GameState.factory
	for _, v := range f.D1().D.VendorsOf(vendor.GetInternalName()) {
		if p := v.Cond(); p != nil {
			if out, err := expr.Run(p, ctx.GameState.questEnv()); err != nil || out != true {
				continue
			}
		}
		for _, sv := range v.Services {
			if item, ok := vendorService(ctx, vendor, sv); ok {
				items = append(items, item)
			}
		}
	}
	return items
}

// vendorService builds one named service; false when it has nothing to offer right now.
func vendorService(ctx ActionContext, vendor *Actor, service string) (util.MenuItem, bool) {
	gui := ctx.UserInterface()
	data := ctx.GameState.factory.D1().D
	name, arg := service, 0
	if util.LooksLikeAFunction(service) {
		n, args := util.GetNameAndArgs(service)
		name, arg = n, args.GetInt(0)
	}
	switch name {
	case "buy":
		return util.MenuItem{MainText: "Buy items", Action: func() { gui.OpenShopMenu(vendor) }}, true
	case "buy_basic":
		return util.MenuItem{MainText: "Buy basic items", Action: func() {
			gui.OpenShopMenuFiltered(vendor, func(item *Item) bool { return !item.IsPremiumStock() })
		}}, true
	case "buy_premium":
		return util.MenuItem{MainText: "Buy premium items", Action: func() {
			gui.OpenShopMenuFiltered(vendor, func(item *Item) bool { return item.IsPremiumStock() })
		}}, true
	case "sell":
		return util.MenuItem{MainText: "Sell items", Action: func() { gui.BeginPlayerSellingTo(vendor) }}, true
	case "repair":
		return d1ServiceOption(ctx, "Repair items", d1.CanRepair, d1.RepairCost, func(it *d1.Item) { it.Durability = it.MaxDur }), true
	case "recharge":
		return d1ServiceOption(ctx, "Recharge staves", data.CanRecharge, data.RechargeCost,
			func(it *d1.Item) { it.Charges = it.MaxCharges }), true
	case "identify": // a flat fee, whatever the item (D1's AddStoreHoldId)
		return d1ServiceOption(ctx, "Identify an item", func(it *d1.Item) bool { return it.Magical != 0 && !it.Identified },
			func(*d1.Item) int { return arg }, func(it *d1.Item) { it.Identified = true; d1IdentifyCue(it) }, true), true
	case "heal": // "Receive healing" (S_HealerEnter): full life, with IS_CAST8 when there was any to restore
		return util.MenuItem{MainText: "Receive healing", Action: func() { healAtVendor(ctx, vendor) }}, true
	case "heal_on_visit": // not a menu entry: coming in is enough
		healAtVendor(ctx, vendor)
	case "donate": // arg gold per explored level refills the dungeon
		levels := ctx.GameState.ExploredD1DungeonLevels()
		if levels == 0 {
			break
		}
		cost := arg * levels
		return util.MenuItem{MainText: fmt.Sprintf("Give her %d gold.", cost), Action: func() {
			if !PayGold(ctx, ctx.GameState.GetPlayer(), cost) {
				return
			}
			n := ctx.GameState.RespawnD1Monsters()
			gui.OpenTextWindow(util.ToLabelText([]string{vendor.Name(), "", "I am very grateful. This will go to a good cause."}))
			ctx.Msg(fmt.Sprintf("Something stirs on %d dungeon levels.", n))
		}}, true
	case "peek": // a look fee of arg, paid once per item: looking again is free until the stock changes
		if vendor.GetInventory().IsEmpty() {
			break
		}
		flags := ctx.GameState.GetFlags()
		label := fmt.Sprintf("I have something for sale, but it will cost %d gold just to take a look", arg)
		if flags.IsSet(FlagWirtItemSeen) {
			label = "What have you got?"
		}
		return util.MenuItem{MainText: label, Action: func() {
			if flags.IsSet(FlagWirtItemSeen) || PayGold(ctx, ctx.GameState.GetPlayer(), arg) {
				flags.SetTrue(FlagWirtItemSeen)
				gui.OpenShopMenu(vendor)
			}
		}}, true
	case "stash":
		return util.MenuItem{MainText: "Open your stash", Action: gui.OpenPlayerStashInTown}, true
	case "transfer":
		return util.MenuItem{MainText: "Transfer an enchantment", Action: gui.OpenTransferEnchantmentMenu}, true
	}
	return util.MenuItem{}, false
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

// d1Broke handles an item whose durability just ran out: like D1's WeaponDur/ArmorDur/ShieldDur, it is destroyed,
// uniques too.
func d1Broke(msg func(string), owner *Actor, broke *d1.Item) {
	if broke == nil {
		return
	}
	PlayUICue("item_break")
	for _, it := range owner.GetEquipment().AllItems() {
		if it.d1 == broke {
			owner.GetEquipment().UnEquip(it)
			owner.GetInventory().Remove(it)
			msg(fmt.Sprintf("Your %s was destroyed.", it.Name()))
		}
	}
	SyncHero(owner)
}

// FlagWirtItemSeen is set once the look fee for Wirt's current item is paid; a new item clears it (saved with the flags).
const FlagWirtItemSeen = "wirt_item_seen"

// exploredD1DungeonNames are the dungeon levels the player has been to: the ones still loaded plus the ones a
// save only kept a snapshot of.
func (s *State) exploredD1DungeonNames() []string {
	seen := map[string]bool{}
	for name := range s.dungeonMaps {
		seen[name] = true
	}
	for name := range s.mapGen.snapshots {
		if !defs.IsTownLevel(name) {
			seen[name] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names) // a Go map's order isn't stable, and the respawn count is shown to the player
	return names
}

// ExploredD1DungeonLevels is how many dungeon levels the player has been to.
func (s *State) ExploredD1DungeonLevels() int { return len(s.exploredD1DungeonNames()) }

// RespawnD1Monsters puts every monster the generator placed back on each explored dungeon level, at full health.
// Levels that are loaded are refilled right away; the rest are marked so they refill the next time they are
// built (LevelSnapshot.RespawnMonsters). Returns how many levels it touched.
func (s *State) RespawnD1Monsters() int {
	g := s.mapGen
	if g.snapshots == nil { // a State built without SetSnapshots still has to be able to mark a level
		g.snapshots = map[string]*LevelSnapshot{}
	}
	names := s.exploredD1DungeonNames()
	for _, name := range names {
		m, loaded := s.dungeonMaps[name]
		lvl := g.d1Levels[name]
		if !loaded || lvl == nil {
			if snap := g.snapshots[name]; snap != nil {
				snap.RespawnMonsters = true
			} else {
				g.snapshots[name] = &LevelSnapshot{Monsters: map[int]SavedMonster{}, RespawnMonsters: true}
			}
			continue
		}
		for _, a := range append([]*Actor(nil), m.Actors()...) {
			if a.d1Monster != nil && !a.d1Monster.Golem {
				m.RemoveActor(a)
			}
		}
		for _, a := range append([]*Actor(nil), m.DownedActors()...) {
			m.RemoveDownedActor(a)
		}
		g.placeD1Monsters(m, lvl)
		for _, a := range append(append([]*Actor(nil), m.Actors()...), g.walledIn[name]...) {
			if a.d1Monster != nil {
				g.factory.initD1Talk(a)
			}
		}
	}
	return len(names)
}

// healAtVendor restores the player's life at a healer.
func healAtVendor(ctx ActionContext, vendor *Actor) {
	player := ctx.GameState.GetPlayer()
	if hp := player.GetStats().GetResource(stats.Health); !hp.IsAtMax() {
		playD1SfxAt(ctx, "IS_CAST8", player.MapPosition())
	}
	player.HealCompletely()
	ctx.Msg(fmt.Sprintf("%s tends to your wounds.", vendor.Name()))
}
