package game

import (
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
)

type gameMap = gridmap.GridMap[*common.Actor, *common.Item, common.Object]

// Decision 10: the H&H town keeps its layout; its shopkeepers become D1's, and Wirt joins.
var d1TownNPCs = map[string]struct {
	name   string
	vendor common.VendorType
}{
	"shekel":  {"Griswold", common.VendorTypeWeapons},
	"kirioth": {"Adria", common.VendorTypeWands},
	"mary":    {"Pepin", common.VendorTypeGeneral},
	"abel":    {"Deckard Cain", common.VendorTypeCain},
}

const wirtIcon = 83

func (g *GameInstance) setupD1Town(m *gameMap) {
	hasWirt := false
	for _, a := range m.Actors() {
		if npc, ok := d1TownNPCs[a.GetInternalName()]; ok {
			a.SetName(npc.name)
			a.SetVendorType(npc.vendor)
		}
		hasWirt = hasWirt || a.GetVendorType() == common.VendorTypeWirt
	}
	if !hasWirt {
		spot := m.GetNamedLocation("to_cathedral")
		if cells := m.GetFreeCellsForDistribution(spot, 3, m.CanPlaceActorHere); len(cells) > 2 {
			wirt := common.NewActor(wirtIcon)
			wirt.SetName("Wirt")
			wirt.SetInternalName("wirt")
			wirt.SetAsciiIcon('w')
			wirt.SetVendorType(common.VendorTypeWirt)
			wirt.SetNoCorpse()
			wirt.SetDrawPosition(cells[2].ToPointF())
			m.AddActor(wirt, cells[2])
		}
	}
	g.restockD1Town(m)
	g.setupD1TownGates(m)
}

// restockD1Town ports SetupTownStores: every visit to town brings new smith, witch and healer stock;
// Griswold's premium items and Wirt's item stay until bought or outleveled.
func (g *GameInstance) restockD1Town(m *gameMap) {
	gen := g.factory.D1()
	player := g.state.GetPlayer()
	lvl := d1.StoreLevel(g.state.GetLastDungeonLevel())
	clvl := player.Hero().Level
	for _, a := range m.Actors() {
		inv := a.GetInventory()
		switch a.GetVendorType() {
		case common.VendorTypeWeapons:
			if g.storesShown {
				for i, it := range g.premium.Items {
					if it != nil && !hasD1Item(inv, it) {
						g.premium.Items[i] = nil // bought
					}
				}
			}
			gen.RefillPremium(&g.premium, clvl)
			g.stock(inv, append(gen.SmithStock(lvl), g.premium.Items[:]...), d1.BuyPrice)
		case common.VendorTypeWands:
			g.stock(inv, gen.WitchStock(lvl), d1.BuyPrice)
		case common.VendorTypeGeneral:
			g.stock(inv, gen.HealerStock(lvl), d1.BuyPrice)
		case common.VendorTypeWirt:
			if g.storesShown && g.boyItem != nil && !hasD1Item(inv, g.boyItem) {
				g.boyItem = nil // bought
			}
			if g.boyItem == nil || g.boyLevel < clvl>>1 {
				g.boyItem, g.boyLevel = gen.BoyItem(clvl), clvl>>1
			}
			g.stock(inv, []*d1.Item{g.boyItem}, d1.WirtPrice)
		}
	}
	g.storesShown = true
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

// onD1MapChanged: the golem and the mana shield don't follow the player to another level (D1 clears missiles and
// the golem on level change), and walking back through the town portal closes it.
func (g *GameInstance) onD1MapChanged(oldMapName, newMapName string) {
	if old := g.state.GetDungeonMap(oldMapName); old != nil {
		for _, a := range append([]*common.Actor(nil), old.Actors()...) {
			if mon := a.D1Monster(); mon != nil && mon.Golem {
				old.RemoveActor(a)
			}
		}
	}
	common.UpdateD1Quests(&g.state, g.state.GetMap())
	player := g.state.GetPlayer()
	player.GetStatusEffects().SetFalse(string(common.StatusManaShield))
	if p := g.state.D1Portal(); p != nil && defs.IsTownLevel(oldMapName) && newMapName == p.Map &&
		geometry.DistanceChebyshev(player.MapPosition(), geometry.Point{X: p.X, Y: p.Y}) <= 1 {
		g.state.CloseD1Portal()
	}
}

// setupD1TownGates opens the town entrances like D1's town warps: the cathedral always, deeper areas once reached.
func (g *GameInstance) setupD1TownGates(m *gameMap) {
	for dlvl, location := range common.D1TownGates {
		if dlvl > 1 && g.state.GetLastDungeonLevel() < dlvl {
			continue
		}
		target := "warp"
		if dlvl == 1 {
			target = "ladder_up"
		}
		pos := m.GetNamedLocation(location)
		m.SetTile(pos, m.GetCell(pos).TileType.WithSpecial(gridmap.SpecialTileStairsDown))
		m.AddTransitionAt(pos, gridmap.Transition{TargetMap: defs.LevelNameFromDepth(dlvl), TargetLocation: target})
	}
}
