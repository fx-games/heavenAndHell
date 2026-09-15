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

func (g *GameInstance) setupD1Town(m *gameMap) {
	has := map[common.VendorType]bool{}
	for _, a := range m.Actors() {
		if npc, ok := d1TownNPCs[a.GetInternalName()]; ok {
			a.SetName(npc.name)
			a.SetVendorType(npc.vendor)
		}
		has[a.GetVendorType()] = true
	}
	// D1's townspeople the H&H town has no one for: Wirt and the wounded townsman by the cathedral, Ogden, Farnham and
	// Gillian at the spawn (D1's tavern)
	for _, npc := range []struct {
		name, internal, near string
		ascii                rune
		vendor               common.VendorType
	}{{"Wirt", "wirt", "to_cathedral", 'w', common.VendorTypeWirt}, {"Wounded Townsman", "deadguy", "to_cathedral", 't', common.VendorTypeDeadGuy},
		{"Ogden", "ogden", "spawn", 'o', common.VendorTypeOgden}, {"Farnham", "farnham", "spawn", 'f', common.VendorTypeDrunk},
		{"Gillian", "gillian", "spawn", 'g', common.VendorTypeBarmaid}} {
		if has[npc.vendor] {
			continue
		}
		cells := freeTownCells(m, m.GetNamedLocation(npc.near))
		if len(cells) == 0 {
			continue
		}
		a := common.NewActor(defs.Icon("npc." + npc.name))
		a.SetName(npc.name)
		a.SetInternalName(npc.internal)
		a.SetAsciiIcon(npc.ascii)
		a.SetVendorType(npc.vendor)
		a.SetNoCorpse()
		a.SetDrawPosition(cells[0].ToPointF())
		m.AddActor(a, cells[0])
	}
	g.restockD1Town(m)
	g.setupD1TownGates(m)
}

// freeTownCells are free cells two or three steps from p, nearest first, so townspeople don't block an entrance.
func freeTownCells(m *gameMap, p geometry.Point) []geometry.Point {
	var cells []geometry.Point
	for r := 2; r <= 3; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if q := p.Add(geometry.Point{X: dx, Y: dy}); geometry.DistanceChebyshev(p, q) == r && m.Contains(q) && m.CanPlaceActorHere(q) {
					cells = append(cells, q)
				}
			}
		}
	}
	return cells
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
			g.stock(inv, gen.HealerStock(lvl), d1.BuyPrice)
		case common.VendorTypeWirt:
			if g.storesShown && g.boyItem != nil && !hasD1Item(inv, g.boyItem) {
				g.boyItem = nil // bought
			}
			if g.boyItem == nil || g.boyLevel < clvl>>1 {
				g.boyItem, g.boyLevel = gen.BoyItem(clvl), clvl>>1
				g.state.GetFlags().SetFalse(common.FlagWirtItemSeen) // a new item costs a new look fee
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
	common.UpdateD1Quests(&g.state, g.state.GetMap())
	g.audioPlayer.PlayMusic(d1AreaMusic(newMapName)) // without that track the current music plays on
	if p := g.state.D1Portal(); p != nil && defs.IsTownLevel(oldMapName) && newMapName == p.Map &&
		geometry.DistanceChebyshev(player.MapPosition(), geometry.Point{X: p.X, Y: p.Y}) <= 1 {
		g.state.CloseD1Portal()
	}
}

// d1AreaMusic names an area's music track, one per area like D1's themes: assets/diablo_music/<name>.ogg
// (dtowne/dlvla-d converted and renamed, see AGENTS.md) if present, else no track is registered under that name
// and the current music just keeps playing.
func d1AreaMusic(mapName string) string {
	if defs.IsTownLevel(mapName) {
		return "town"
	}
	return [4]string{"cathedral", "catacombs", "caves", "hell"}[min(max(common.LevelFromName(mapName)-1, 0)/4, 3)]
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
