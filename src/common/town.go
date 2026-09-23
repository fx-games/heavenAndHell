package common

import (
	"fmt"

	"github.com/expr-lang/expr"

	"rpg/src/rules"
	"rpg/src/stats"
	"rpg/src/util"
	"sort"
)

// D1 townspeople (decision 10). A vendor type is the NPC's InternalName on the map and its gossip key in rules.Data.Gossip.
const (
	VendorTypeCain    VendorType = "cain"
	VendorTypeWirt    VendorType = "wirt"
	VendorTypeOgden   VendorType = "ogden"   // no store in single player, only talk
	VendorTypeDeadGuy VendorType = "deadguy" // the wounded townsman: only his last words
	VendorTypeDrunk   VendorType = "farnham" // only talk (S_StartDrunk)
	VendorTypeBarmaid VendorType = "gillian" // only talk (S_StartBarmaid)
)

// VendorBuys is whether vendor takes item off the hero: its vendor records' buys rule (SmithSellOk,
// WitchSellOk, or anything).
func VendorBuys(vendor *Actor, item *Item) bool {
	if item.roll == nil || item.data == nil {
		return false
	}
	for _, v := range item.data.VendorsOf(vendor.GetInternalName()) {
		switch v.Buys {
		case "smith":
			return item.data.SmithBuys(item.roll)
		case "witch":
			return item.data.WitchBuys(item.roll)
		case "hellfire_smith":
			return item.data.HellfireSmithBuys(item.roll)
		case "hellfire_witch":
			return item.data.HellfireWitchBuys(item.roll)
		case "any":
			return true
		}
	}
	return false
}

// Shopper is what stores with Hellfire's options look at of the player (SpawnOnePremium, SpawnBoy, HealerItemOk).
func Shopper(player *Actor) *rules.Shopper {
	h, s := player.hero, player.HeroSheet()
	if h == nil {
		return nil
	}
	who := &rules.Shopper{Class: h.Class, Str: s.Str, Mag: s.Mag, Dex: s.Dex, BaseStr: h.BaseStr, BaseMag: h.BaseMag,
		BaseDex: h.BaseDex, BaseVit: h.BaseVit}
	for _, it := range heroItems(player) {
		if it.roll != nil {
			who.Owned = append(who.Owned, it.roll)
		}
	}
	return who
}

func SellPrice(item *Item) int { return rules.SellPrice(item.roll) }

// PayGold takes gold from the player; false (with a message) if there isn't enough.
func PayGold(ctx ActionContext, player *Actor, cost int) bool {
	if player.GetStats().GetAttribute(stats.Gold) < cost {
		ctx.Msg("You don't have enough gold.")
		return false
	}
	player.GetStats().DecrementBy(stats.Gold, cost)
	return true
}

// serviceOption lists the player's items a service applies to, with prices, and performs it on selection. Like the
// sell menu, the list reopens after each purchase until nothing is left for the service.
// showAfter, when set, opens the item's description on top of the reopened list (or the NPC's menu, once nothing is
// left for the service), so closing it goes back there.
func serviceOption(ctx ActionContext, label string, applies func(*rules.Item) bool, cost func(*rules.Item) int, perform func(*rules.Item), showAfter ...bool) util.MenuItem {
	var open func(reopen bool)
	open = func(reopen bool) {
		player := ctx.GameState.GetPlayer()
		var items []*Item
		for _, item := range heroItems(player) {
			if item.roll != nil && applies(item.roll) {
				items = append(items, item)
			}
		}
		if len(items) == 0 {
			if !reopen {
				ctx.Msg("You have nothing for that.")
			}
			return
		}
		ctx.Show(ChooseService{Items: items, Price: func(it *Item) int { return cost(it.roll) }, Pick: func(it *Item) {
			price := cost(it.roll)
			done := PayGold(ctx, player, price)
			if done {
				perform(it.roll)
				it.Refresh()
				SyncHero(player)
				ctx.Msg(fmt.Sprintf("%s: done (%d gold)", it.Name(), price))
			}
			open(true)
			if done && len(showAfter) > 0 && showAfter[0] {
				ctx.Show(ShowItem{it})
			}
		}})
	}
	return util.MenuItem{MainText: label, Action: func() { open(false) }}
}

// vendorOptions is the town NPC's menu: the services of each of its vendor records whose condition holds, in
// stores.rec order. Every service is implemented here once; a mod only picks which ones an NPC offers. D1's
// labels are kept (S_StartSmith, S_StartWitch, S_StartHealer, S_StartStory, S_StartBoy).
func vendorOptions(ctx ActionContext, vendor *Actor) []util.MenuItem {
	var items []util.MenuItem
	f := ctx.GameState.factory
	for _, v := range f.Gen().D.VendorsOf(vendor.GetInternalName()) {
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
	data := ctx.GameState.factory.Gen().D
	name, arg := service, 0
	if util.LooksLikeAFunction(service) {
		n, args := util.GetNameAndArgs(service)
		name, arg = n, args.GetInt(0)
	}
	switch name {
	case "buy":
		return util.MenuItem{MainText: "Buy items", Action: func() { ctx.Show(OpenShop{Vendor: vendor}) }}, true
	case "buy_basic":
		return util.MenuItem{MainText: "Buy basic items", Action: func() {
			ctx.Show(OpenShop{vendor, func(item *Item) bool { return !item.IsPremiumStock() }})
		}}, true
	case "buy_premium":
		return util.MenuItem{MainText: "Buy premium items", Action: func() {
			ctx.Show(OpenShop{vendor, func(item *Item) bool { return item.IsPremiumStock() }})
		}}, true
	case "sell":
		return util.MenuItem{MainText: "Sell items", Action: func() { ctx.Show(OpenSelling{vendor}) }}, true
	case "repair":
		return serviceOption(ctx, "Repair items", rules.CanRepair, rules.RepairCost, func(it *rules.Item) { it.Durability = it.MaxDur }), true
	case "recharge":
		return serviceOption(ctx, "Recharge staves", data.CanRecharge, data.RechargeCost,
			func(it *rules.Item) { it.Charges = it.MaxCharges }), true
	case "identify": // a flat fee, whatever the item (D1's AddStoreHoldId)
		return serviceOption(ctx, "Identify an item", func(it *rules.Item) bool { return it.Magical != 0 && !it.Identified },
			func(*rules.Item) int { return arg }, func(it *rules.Item) { it.Identified = true; ctx.ActionBroker.Emit(ItemIdentified{Item: it}) }, true), true
	case "heal": // "Receive healing" (S_HealerEnter): full life, with IS_CAST8 when there was any to restore
		return util.MenuItem{MainText: "Receive healing", Action: func() { healAtVendor(ctx, vendor) }}, true
	case "heal_on_visit": // not a menu entry: coming in is enough
		healAtVendor(ctx, vendor)
	case "donate": // arg gold per explored level refills the dungeon
		levels := ctx.GameState.ExploredDungeonLevels()
		if levels == 0 {
			break
		}
		cost := arg * levels
		return util.MenuItem{MainText: fmt.Sprintf("Give her %d gold.", cost), Action: func() {
			if !PayGold(ctx, ctx.GameState.GetPlayer(), cost) {
				return
			}
			n := ctx.GameState.RespawnMonsters()
			ctx.Show(ShowText{[]string{vendor.Name(), "", "I am very grateful. This will go to a good cause."}})
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
				ctx.Show(OpenShop{Vendor: vendor})
			}
		}}, true
	case "stash":
		return util.MenuItem{MainText: "Open your stash", Action: func() { ctx.Show(OpenStash{}) }}, true
	case "transfer":
		return util.MenuItem{MainText: "Transfer an enchantment", Action: func() { ctx.Show(OpenEnchantmentTransfer{}) }}, true
	}
	return util.MenuItem{}, false
}

func (a *Actor) equipped() []*rules.Item {
	var items []*rules.Item
	for _, it := range a.GetEquipment().AllItems() {
		if it.roll != nil {
			items = append(items, it.roll)
		}
	}
	return items
}

// itemBroke handles an item whose durability just ran out: like D1's WeaponDur/ArmorDur/ShieldDur, it is destroyed,
// uniques too.
func itemBroke(b *Broker, msg func(string), owner *Actor, broke *rules.Item) {
	if broke == nil {
		return
	}
	b.Emit(ItemBroke{Owner: owner})
	for _, it := range owner.GetEquipment().AllItems() {
		if it.roll == broke {
			owner.GetEquipment().UnEquip(it)
			owner.GetInventory().Remove(it)
			msg(fmt.Sprintf("Your %s was destroyed.", it.Name()))
		}
	}
	SyncHero(owner)
}

// FlagWirtItemSeen is set once the look fee for Wirt's current item is paid; a new item clears it (saved with the flags).
const FlagWirtItemSeen = "wirt_item_seen"

// exploredDungeonNames are the dungeon levels the player has been to: the ones still loaded plus the ones a
// save only kept a snapshot of.
func (s *State) exploredDungeonNames() []string {
	seen := map[string]bool{}
	for name := range s.maps {
		seen[name] = !s.Level(name).Town
	}
	for name := range s.mapGen.snapshots {
		seen[name] = !s.Level(name).Town
	}
	for name, below := range seen {
		if !below {
			delete(seen, name)
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names) // a Go map's order isn't stable, and the respawn count is shown to the player
	return names
}

// ExploredDungeonLevels is how many dungeon levels the player has been to.
func (s *State) ExploredDungeonLevels() int { return len(s.exploredDungeonNames()) }

// RespawnMonsters puts every monster the generator placed back on each explored dungeon level, at full health.
// Levels that are loaded are refilled right away; the rest are marked so they refill the next time they are
// built (LevelSnapshot.RespawnMonsters). Returns how many levels it touched.
func (s *State) RespawnMonsters() int {
	g := s.mapGen
	if g.snapshots == nil { // a State built without SetSnapshots still has to be able to mark a level
		g.snapshots = map[string]*LevelSnapshot{}
	}
	names := s.exploredDungeonNames()
	for _, name := range names {
		m, loaded := s.maps[name]
		lvl := g.generated[name]
		if !loaded || lvl == nil {
			if snap := g.snapshots[name]; snap != nil {
				snap.RespawnMonsters = true
			} else {
				g.snapshots[name] = &LevelSnapshot{Monsters: map[int]SavedMonster{}, RespawnMonsters: true}
			}
			continue
		}
		for _, a := range append([]*Actor(nil), m.Actors()...) {
			if a.monster != nil && !a.monster.Golem {
				m.RemoveActor(a)
			}
		}
		for _, a := range append([]*Actor(nil), m.DownedActors()...) {
			m.RemoveDownedActor(a)
		}
		g.placeMonsters(m, lvl)
		for _, a := range append(append([]*Actor(nil), m.Actors()...), g.walledIn[name]...) {
			if a.monster != nil {
				g.factory.initTalk(a)
			}
		}
	}
	return len(names)
}

// healAtVendor restores the player's life at a healer.
func healAtVendor(ctx ActionContext, vendor *Actor) {
	player := ctx.GameState.GetPlayer()
	if hp := player.GetStats().GetResource(stats.Health); !hp.IsAtMax() {
		playSfxAt(ctx, "IS_CAST8", player.MapPosition())
	}
	player.HealCompletely()
	ctx.Msg(fmt.Sprintf("%s tends to your wounds.", vendor.Name()))
}
