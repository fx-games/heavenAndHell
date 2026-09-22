package d1

// items.cpp: items placed on level creation.

const (
	maxItems = 127
)

// PlacedItem is an item lying on the level, in piece coordinates.
type PlacedItem struct {
	X, Y    int
	Item    *Item
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
func (g *gen) placeRolledItem(x, y int, roll func() *Item) {
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
	var idx int
	if onlygood {
		idx = g.igen.rndUItem(nil, 0, g.currlevel)
	} else {
		idx = g.igen.rndAllItems(g.currlevel)
	}
	g.placeRolledItem(x, y, func() *Item {
		return g.igen.setupAllItems(idx, g.rng.Seed(), 2*g.currlevel, g.currlevel, 1, onlygood, false, true, g.ltype == DTypeHell)
	})
}

// createTypeItem ports CreateTypeItem.
func (g *gen) createTypeItem(x, y int, onlygood bool, itype int, imisc string) {
	idx := IdxGold
	if itype != ITypeGold {
		idx = g.igen.rndTypeItems(itype, imisc, g.currlevel)
	}
	g.placeRolledItem(x, y, func() *Item {
		return g.igen.setupAllItems(idx, g.rng.Seed(), 2*g.currlevel, g.currlevel, 1, onlygood, false, true, g.ltype == DTypeHell)
	})
}

// spawnQuestItem ports SpawnQuestItem at a fixed cell.
func (g *gen) spawnQuestItem(idx, x, y, selflag int) {
	if g.numitems >= maxItems {
		return
	}
	p := &PlacedItem{X: x, Y: y, SelFlag: selflag}
	ii := g.newItemSlot(p)
	g.setDItem(x, y, ii+1)
	p.Item = g.igen.itemAttrs(idx, g.currlevel, g.currlevel, g.ltype == DTypeHell)
	g.numitems++
}

// initItems ports InitItems.
func (g *gen) initItems() {
	g.igen.itemAttrs(IdxGold, 1, g.currlevel, g.ltype == DTypeHell) // golditem
	g.clearItems()
	if g.setlevel {
		return
	}
	g.rng.Seed()
	if g.available(QRock) {
		for _, oi := range g.objectactive[:g.nobjects] {
			if o := g.objects[oi]; o.Kind == ObjSTAND {
				g.spawnQuestItem(IdiRock, o.X, o.Y, 2)
				break
			}
		}
	}
	if g.available(QAnvil) {
		g.spawnQuestItem(IdiAnvil, 2*g.setPiece.x+27, 2*g.setPiece.y+27, 1)
	}
	if g.currlevel > 0 && g.currlevel < 16 {
		g.addInitItems()
	}
}

// questItemSpot ports SpawnQuestItem's randarea: a random spot with randarea×randarea free cells, shrinking the
// area after 1000 tries.
func (g *gen) questItemSpot(randarea int) (x, y int) {
	for tries := 1; ; tries++ {
		if tries > 1000 && randarea > 1 {
			randarea--
		}
		x, y = g.rnd(112), g.rnd(112)
		ok := true
		for i := 0; i < randarea && ok; i++ {
			for j := 0; j < randarea && ok; j++ {
				ok = g.itemSpaceOk(i+x, j+y)
			}
		}
		if ok {
			return
		}
	}
}

// addInitItems ports AddInitItems: 3-5 potions lying around.
func (g *gen) addInitItems() {
	for n := g.rnd(3) + 3; n > 0; n-- {
		p := &PlacedItem{SelFlag: 1}
		ii := g.newItemSlot(p)
		x, y := g.rnd(80)+16, g.rnd(80)+16
		for !g.itemPlace(x, y) {
			x, y = g.rnd(80)+16, g.rnd(80)+16
		}
		p.X, p.Y = x, y
		g.setDItem(x, y, ii+1)
		seed := g.rng.Seed()
		g.rng.SetSeed(seed)
		idx := IdxMana
		if g.rnd(2) != 0 {
			idx = IdxHeal
		}
		p.Item = g.igen.itemAttrs(idx, g.currlevel, g.currlevel, false)
		p.Item.Seed, p.Item.CreateInfo = seed, g.currlevel|CFPregen
		g.numitems++
	}
}
