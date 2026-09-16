package game

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
)

type gameMap = gridmap.GridMap[*common.Actor, *common.Item, common.Object]

// restockD1Town ports SetupTownStores: every visit to town brings new smith, witch and healer stock;
// Griswold's premium items and Wirt's item stay until bought or outleveled.
func (g *GameInstance) restockD1Town(m *gameMap) {
	gen := g.factory.D1()
	player := g.state.GetPlayer()
	who := common.D1Shopper(player)
	lvl := d1.StoreLevel(g.state.GetLastDungeonLevel())
	clvl := player.Hero().Level
	g.forgetBoughtD1Stock(m)
	for _, a := range m.Actors() {
		inv := a.GetInventory()
		switch a.GetVendorType() {
		case common.VendorTypeWeapons:
			gen.RefillPremium(&g.premium, clvl, who)
			g.stock(inv, append(gen.SmithStock(lvl), g.premium.Items[:]...), d1.BuyPrice)
			for _, item := range inv.Items() {
				for _, premiumItem := range g.premium.Items {
					if premiumItem != nil && item.D1() == premiumItem {
						item.SetPremiumStock(true)
					}
				}
			}
		case common.VendorTypeWands:
			g.stock(inv, gen.WitchStock(lvl, player.Hero().SpellLevels), d1.BuyPrice)
		case common.VendorTypeGeneral:
			g.stock(inv, gen.HealerStock(lvl, who), d1.BuyPrice)
		case common.VendorTypeWirt:
			if g.boyItem == nil || g.boyLevel < clvl>>1 {
				g.boyItem, g.boyLevel = gen.BoyItem(clvl, who), clvl>>1
				g.state.GetFlags().SetFalse(common.FlagWirtItemSeen) // a new item costs a new look fee
			}
			g.stock(inv, []*d1.Item{g.boyItem}, gen.D.WirtPrice)
		}
	}
	g.storesShown = true
}

// forgetBoughtD1Stock clears premium and Wirt items missing from their vendor's inventory. It runs before every
// restock and save, so a save never brings back an item that was already bought.
func (g *GameInstance) forgetBoughtD1Stock(town *gameMap) {
	if !g.storesShown || town == nil { // the town inventories don't hold the saved stock yet
		return
	}
	for _, a := range town.Actors() {
		inv := a.GetInventory()
		switch a.GetVendorType() {
		case common.VendorTypeWeapons:
			for i, it := range g.premium.Items {
				if it != nil && !hasD1Item(inv, it) {
					g.premium.Items[i] = nil
				}
			}
		case common.VendorTypeWirt:
			if g.boyItem != nil && !hasD1Item(inv, g.boyItem) {
				g.boyItem = nil
			}
		}
	}
}

func hasD1Item(inv *common.Inventory, it *d1.Item) bool {
	for _, item := range inv.Items() {
		if item.D1() == it {
			return true
		}
	}
	return false
}

func (g *GameInstance) stock(inv *common.Inventory, items []*d1.Item, price func(*d1.Item) int) {
	for _, old := range append([]*common.Item(nil), inv.Items()...) {
		inv.Remove(old)
	}
	for _, it := range items {
		if it != nil {
			item := g.factory.NewD1Item(it)
			item.SetBaseCost(price(it))
			inv.Add(item)
		}
	}
}

// onD1MapChanged: the golem follows the player to the new level and the mana shield stays up (unlike D1, which
// clears both on a level change), and walking back through the town portal closes it.
func (g *GameInstance) onD1MapChanged(oldMapName, newMapName string) {
	player := g.state.GetPlayer()
	old := g.state.GetDungeonMap(oldMapName)
	if old == nil {
		old = g.state.GetTownMap(oldMapName)
	}
	if old != nil && old != g.state.GetMap() {
		if golem := common.D1Golem(old); golem != nil {
			old.RemoveActor(golem)
			g.factory.PlaceD1GolemNear(player, common.SaveD1Golem(golem))
		}
	}
	if h := player.Hero(); h != nil {
		h.Reflections = 0 // FreeMissiles: Reflect ends with the level
	}
	common.UpdateD1Quests(&g.state, g.state.GetMap())
	g.audioPlayer.PlayMusic(d1AreaMusic(newMapName)) // without that track the current music plays on
	if p := g.state.D1Portal(); p != nil && defs.IsTownLevel(oldMapName) && newMapName == p.Map &&
		geometry.DistanceChebyshev(player.MapPosition(), geometry.Point{X: p.X, Y: p.Y}) <= 1 {
		g.state.CloseD1Portal()
	}
}

// d1AreaMusic names an area's music track, one per area like D1's themes: assets/diablo_music/<name>.ogg
// (dtowne/dlvla-d converted and renamed, see AGENTS.md) if present, else no track is registered under that name
// and the current music just keeps playing. Hellfire's two areas come from hfmusic.mpq as nest/crypt.
func d1AreaMusic(mapName string) string {
	if defs.IsTownLevel(mapName) {
		return "town"
	}
	dlvl := max(common.LevelFromName(mapName), 1)
	switch {
	case dlvl >= 21: // the Crypt
		return "crypt"
	case dlvl >= 17: // the Festering Nest
		return "nest"
	}
	return [4]string{"cathedral", "catacombs", "caves", "hell"}[min((dlvl-1)/4, 3)]
}
