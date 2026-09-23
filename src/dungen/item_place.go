package dungen

import "rpg/src/rules"

// items.cpp: items placed on level creation.

const (
	maxItems = 127
)

// PlacedItem is an item lying on the level, in piece coordinates.
type PlacedItem struct {
	X, Y    int
	Item    *rules.Item
	SelFlag int // 2: can only be picked up from its object (the Magic Rock on its stand)
}

func (g *gen) clearItems() {
	g.itemSlots = [maxItems]*PlacedItem{}
	g.numitems = 0
	for i := range g.itemavail {
		g.itemavail[i] = i
		g.itemactive[i] = 0
	}
}

// newItemSlot takes D1's next free item slot.
func (g *gen) newItemSlot(p *PlacedItem) int {
	ii := g.itemavail[0]
	g.itemavail[0] = g.itemavail[maxItems-g.numitems-1]
	g.itemactive[g.numitems] = ii
	g.itemSlots[ii] = p
	return ii
}

func (g *gen) setDItem(x, y, v int) {
	if x, y, ok := flat(x, y); ok {
		g.dItem[x][y] = int8(v)
	}
}

// itemPlace ports ItemPlace.
func (g *gen) itemPlace(x, y int) bool {
	return g.dMon(x, y) == 0 && g.dIt(x, y) == 0 && g.dObj(x, y) == 0 && g.flagAt(x, y)&bflagPopulated == 0 && !g.solid(x, y)
}

// itemSpaceOk ports ItemSpaceOk.
func (g *gen) itemSpaceOk(i, j int) bool {
	if i < 0 || i >= 112 || j < 0 || j >= 112 || g.dMonster[i][j] != 0 || g.dItem[i][j] != 0 {
		return false
	}
	if o := g.objAt(i, j); o != nil && o.Solid {
		return false
	}
	if o := g.objAt(i+1, j+1); o != nil && o.SelFlag != 0 {
		return false
	}
	if a, b := g.dObj(i+1, j), g.dObj(i, j+1); a > 0 && b > 0 && g.objects[a-1].SelFlag != 0 && g.objects[b-1].SelFlag != 0 {
		return false
	}
	return !g.solid(i, j)
}

// getSuperItemSpace ports GetSuperItemSpace (with GetItemSpace): a free cell around (x, y) for a dropped item.
func (g *gen) getSuperItemSpace(x, y int) (int, int) {
	var hold [3][3]bool
	avail := false
	for j := 0; j < 3; j++ {
		for i := 0; i < 3; i++ {
			hold[i][j] = g.itemSpaceOk(x-1+i, y-1+j)
			avail = avail || hold[i][j]
		}
	}
	rs := g.rnd(15) + 1
	if avail {
		xx, yy := 0, 0
		for rs > 0 {
			if hold[xx][yy] {
				rs--
			}
			if rs > 0 {
				if xx++; xx == 3 {
					xx = 0
					if yy++; yy == 3 {
						yy = 0
					}
				}
			}
		}
		return xx + x - 1, yy + y - 1
	}
	for k := 2; k < 50; k++ {
		for j := -k; j <= k; j++ {
			for i := -k; i <= k; i++ {
				if g.itemSpaceOk(i+x, j+y) {
					return i + x, j + y
				}
			}
		}
	}
	return 0, 0
}

// placeRolledItem puts an item in a slot at a cell found by getSuperItemSpace, rolling it after the cell.
func (g *gen) placeRolledItem(x, y int, roll func() *rules.Item) {
	if g.numitems >= maxItems {
		return
	}
	p := &PlacedItem{}
	ii := g.newItemSlot(p)
	p.X, p.Y = g.getSuperItemSpace(x, y)
	g.setDItem(p.X, p.Y, ii+1)
	p.Item = roll()
	p.SelFlag = 1
	g.numitems++
}

// createRndItem ports CreateRndItem (with the level's pregen flag).
func (g *gen) createRndItem(x, y int, onlygood bool) {
	var b *rules.ItemData
	if onlygood {
		b, _ = g.igen.RndUItem(nil, 0, g.currlevel)
	} else {
		b = g.igen.RndAllItems(g.currlevel)
	}
	if b == nil {
		return
	}
	g.placeRolledItem(x, y, func() *rules.Item {
		return g.igen.SetupAllItems(b, g.rng.Seed(), 2*g.currlevel, g.currlevel, 1, onlygood, false, true, g.ltype == rules.DTypeHell)
	})
}

// createTypeItem ports CreateTypeItem.
func (g *gen) createTypeItem(x, y int, onlygood bool, itype int, imisc string) {
	b := g.igen.RndTypeItems(itype, imisc, g.currlevel)
	if b == nil {
		return
	}
	g.placeRolledItem(x, y, func() *rules.Item {
		return g.igen.SetupAllItems(b, g.rng.Seed(), 2*g.currlevel, g.currlevel, 1, onlygood, false, true, g.ltype == rules.DTypeHell)
	})
}

// spawnQuestItem ports SpawnQuestItem at a fixed cell.
func (g *gen) spawnQuestItem(name string, x, y, selflag int) {
	if g.numitems >= maxItems {
		return
	}
	p := &PlacedItem{X: x, Y: y, SelFlag: selflag}
	ii := g.newItemSlot(p)
	g.setDItem(x, y, ii+1)
	p.Item = g.igen.ItemAttrs(g.igen.D.Item(name), g.currlevel, g.currlevel, g.ltype == rules.DTypeHell)
	g.numitems++
}

// initItems ports InitItems.
func (g *gen) initItems() {
	g.clearItems()
	if g.setlevel {
		return
	}
	g.rng.Seed()
	if g.available(rules.QRock) {
		for _, oi := range g.objectactive[:g.nobjects] {
			if o := g.objects[oi]; o.Kind == rules.ObjSTAND {
				g.spawnQuestItem(rules.ItemRock, o.X, o.Y, 2)
				break
			}
		}
	}
	if g.cells != nil {
		g.pieceItems()
	} else if g.available(rules.QAnvil) {
		g.spawnQuestItem(rules.ItemAnvil, 2*g.setPiece.x+27, 2*g.setPiece.y+27, 1)
	}
	if g.currlevel > 0 && g.currlevel < 16 {
		g.addInitItems()
	}
}

// addInitItems ports AddInitItems: the mod's level table, rolled count times, lies around the level (1.09: 3-5
// healing or mana potions).
func (g *gen) addInitItems() {
	t := g.igen.D.DropTables["level"]
	if len(t.Drops) == 0 {
		return
	}
	for n := g.rnd(t.Count.Max()-t.Count.Min()+1) + t.Count.Min(); n > 0; n-- {
		seed := g.rng.Seed()
		g.rng.SetSeed(seed)
		b := g.igen.RollDrop("level", g.currlevel, func() *rules.ItemData { return g.igen.AnyFloorDrop(g.currlevel) })
		if b == nil {
			continue
		}
		p := &PlacedItem{SelFlag: 1}
		ii := g.newItemSlot(p)
		x, y := g.rnd(80)+16, g.rnd(80)+16
		for !g.itemPlace(x, y) {
			x, y = g.rnd(80)+16, g.rnd(80)+16
		}
		p.X, p.Y = x, y
		g.setDItem(x, y, ii+1)
		p.Item = g.igen.ItemAttrs(b, g.currlevel, g.currlevel, false)
		p.Item.Seed, p.Item.CreateInfo = seed, g.currlevel|rules.CFPregen
		g.numitems++
	}
}
