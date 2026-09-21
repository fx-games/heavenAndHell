package game

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"slices"
)

type gameMap = gridmap.GridMap[*common.Actor, *common.Item, common.Object]

// restockD1Town ports SetupTownStores for every mod: each town NPC's stock is its vendor records' stock rolls, in
// stores.rec order. Premium items and Wirt's item stay until bought or outleveled; everything else is new each visit.
func (g *GameInstance) restockD1Town(m *gameMap) {
	gen := g.factory.D1()
	player := g.state.GetPlayer()
	who := common.D1Shopper(player)
	lvl := d1.StoreLevel(g.state.GetLastDungeonLevel())
	clvl := player.Hero().Level
	g.forgetBoughtD1Stock(m)
	for _, a := range m.Actors() {
		var stock, premium []*d1.Item
		price := d1.BuyPrice
		for _, v := range gen.D.VendorsOf(a.GetInternalName()) {
			switch v.Stock {
			case "premium":
				gen.RefillPremium(&g.premium, clvl, who)
				premium = g.premium.Items[:]
			case "smith":
				stock = append(stock, gen.SmithStock(lvl)...)
			case "witch":
				stock = append(stock, gen.WitchStock(lvl, player.Hero().SpellLevels)...)
			case "healer":
				stock = append(stock, gen.HealerStock(lvl, who)...)
			case "list":
				stock = append(stock, gen.ListStock(v, lvl)...)
			case "boy":
				if g.boyItem == nil || g.boyLevel < clvl>>1 {
					g.boyItem, g.boyLevel = gen.BoyItem(clvl, who), clvl>>1
					g.state.GetFlags().SetFalse(common.FlagWirtItemSeen) // a new item costs a new look fee
				}
				stock, price = append(stock, g.boyItem), gen.D.WirtPrice
			default:
				continue
			}
		}
		if stock == nil && premium == nil {
			continue
		}
		inv := a.GetInventory()
		g.stock(inv, append(stock, premium...), price)
		for _, item := range inv.Items() {
			if slices.Contains(premium, item.D1()) {
				item.SetPremiumStock(true)
			}
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
		for _, v := range g.factory.D1().D.VendorsOf(a.GetInternalName()) {
			switch v.Stock {
			case "premium":
				for i, it := range g.premium.Items {
					if it != nil && !hasD1Item(inv, it) {
						g.premium.Items[i] = nil
					}
				}
			case "boy":
				if g.boyItem != nil && !hasD1Item(inv, g.boyItem) {
					g.boyItem = nil
				}
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

// d1AreaMusic names an area's music track, one per area like D1's themes. The tracks come from
// assets/data/music.rec (./remap_music.sh); a theme with no file assigned isn't registered under that name and the
// current music just keeps playing.
func d1AreaMusic(mapName string) string {
	if defs.IsTownLevel(mapName) {
		return "town"
	}
	dlvl := max(common.LevelFromName(mapName), 1)
	return [4]string{"cathedral", "catacombs", "caves", "hell"}[min((dlvl-1)/4, 3)]
}
