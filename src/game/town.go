package game

import (
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/rules"
	"slices"
)

type gameMap = gridmap.GridMap[*common.Actor, *common.Item, common.Object]

// restockTown ports SetupTownStores for every mod: each town NPC's stock is its vendor records' stock rolls, in
// stores.rec order. Premium items and Wirt's item stay until bought or outleveled; everything else is new each visit.
func (g *GameInstance) restockTown(m *gameMap) {
	gen := g.factory.Gen()
	player := g.state.GetPlayer()
	who := common.Shopper(player)
	lvl := rules.StoreLevel(max(g.state.Deepest(), 1))
	clvl := player.Hero().Level
	g.forgetBoughtStock(m)
	for _, a := range m.Actors() {
		var stock, premium []*rules.Item
		price := rules.BuyPrice
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
			if slices.Contains(premium, item.Roll()) {
				item.SetPremiumStock(true)
			}
		}
	}
	g.storesShown = true
}

// forgetBoughtStock clears premium and Wirt items missing from their vendor's inventory. It runs before every
// restock and save, so a save never brings back an item that was already bought.
func (g *GameInstance) forgetBoughtStock(town *gameMap) {
	if !g.storesShown || town == nil { // the town inventories don't hold the saved stock yet
		return
	}
	for _, a := range town.Actors() {
		inv := a.GetInventory()
		for _, v := range g.factory.Gen().D.VendorsOf(a.GetInternalName()) {
			switch v.Stock {
			case "premium":
				for i, it := range g.premium.Items {
					if it != nil && !hasItem(inv, it) {
						g.premium.Items[i] = nil
					}
				}
			case "boy":
				if g.boyItem != nil && !hasItem(inv, g.boyItem) {
					g.boyItem = nil
				}
			}
		}
	}
}

func hasItem(inv *common.Inventory, it *rules.Item) bool {
	for _, item := range inv.Items() {
		if item.Roll() == it {
			return true
		}
	}
	return false
}

func (g *GameInstance) stock(inv *common.Inventory, items []*rules.Item, price func(*rules.Item) int) {
	for _, old := range append([]*common.Item(nil), inv.Items()...) {
		inv.Remove(old)
	}
	for _, it := range items {
		if it != nil {
			item := g.factory.NewRolledItem(it)
			item.SetBaseCost(price(it))
			inv.Add(item)
		}
	}
}

// onDungeonMapChanged ports RemovePlrMissiles for a level change: the golem dies where it stood, leaving its corpse, and
// the mana shield ends (1.09's InitLevelChange; Hellfire kept it). Walking back through the town portal closes it.
func (g *GameInstance) onDungeonMapChanged(oldMapName, newMapName string) {
	player := g.state.GetPlayer()
	old := g.state.Map(oldMapName)
	if old != nil && old != g.state.GetMap() {
		if golem := common.LivingGolem(old); golem != nil {
			old.RemoveActor(golem)
			old.AddDownedActor(golem, golem.MapPosition())
		}
	}
	if h := player.Hero(); h != nil {
		h.Reflections = 0 // FreeMissiles: Reflect ends with the level
	}
	player.RemoveStatusEffect(common.StatusManaShield)
	common.Arrive(&g.state, g.state.GetMap())
	g.playLevelMusic()
	if p := g.state.Portal(); p != nil && g.state.Level(oldMapName).Town && newMapName == p.Map &&
		geometry.DistanceChebyshev(player.MapPosition(), geometry.Point{X: p.X, Y: p.Y}) <= 1 {
		g.state.ClosePortal()
	}
}

// playLevelMusic plays the level's own music.rec event (levels.rec's music); a level without one is silent.
func (g *GameInstance) playLevelMusic() {
	if music := g.state.CurrentLevel().Music; music != "" {
		g.audioPlayer.PlayMusic(music)
	} else {
		g.audioPlayer.StopMusic()
	}
}
