package d1

// setmaps.cpp: the quest set levels.

var (
	skelKingTrans1 = []int{19, 47, 26, 55, 26, 49, 30, 53}
	skelKingTrans2 = []int{33, 19, 47, 29, 37, 29, 43, 39}
	skelKingTrans3 = []int{27, 53, 35, 61, 27, 35, 34, 42, 45, 35, 53, 43, 45, 53, 53, 61, 31, 39, 49, 57}
	skelKingTrans4 = []int{49, 45, 58, 51, 57, 31, 62, 37, 63, 31, 69, 40, 59, 41, 73, 55, 63, 55, 69, 65, 73, 45, 78, 51, 79, 43, 89, 53}
	skelChamTrans1 = []int{43, 19, 50, 26, 51, 19, 59, 26, 35, 27, 42, 34, 43, 27, 49, 34, 50, 27, 59, 34}
	skelChamTrans2 = []int{19, 31, 34, 47, 34, 35, 42, 42}
	skelChamTrans3 = []int{43, 35, 50, 42, 51, 35, 62, 42, 63, 31, 66, 46, 67, 31, 78, 34, 67, 35, 78, 42, 67, 43, 78, 46, 35, 43, 42, 51, 43, 43, 49, 51, 50, 43, 59, 51}
)

// listTrans ports DRLG_ListTrans: one room region per rectangle.
func (g *gen) listTrans(list []int) {
	for i := 0; i+3 < len(list); i += 4 {
		g.rectTrans(list[i], list[i+1], list[i+2], list[i+3])
	}
}

// areaTrans ports DRLG_AreaTrans: the rectangles share one region.
func (g *gen) areaTrans(list []int) {
	for i := 0; i+3 < len(list); i += 4 {
		g.rectTrans(list[i], list[i+1], list[i+2], list[i+3])
		g.transVal--
	}
	g.transVal++
}

// loadSetTiles writes a set level's tiles like LoadPreL*/LoadL*Dungeon: dirt everywhere, the .DUN on top with its
// empty cells as floor.
func (g *gen) loadSetTiles(d dun, floor, dirt byte, protect bool) {
	g.fillDungeon(dirt)
	g.protected = [1600]bool{}
	for j := 0; j < d.h(); j++ {
		for i := 0; i < d.w(); i++ {
			if t := d.tile(i, j); t != 0 {
				g.sd(i, j, t)
				if protect {
					g.setProt(i, j)
				}
			} else {
				g.sd(i, j, floor)
			}
		}
	}
}

// loadPreL1 ports LoadPreL1Dungeon: the layout before the quest changes it, kept in pdungeon.
func (g *gen) loadPreL1(name string) {
	g.loadSetTiles(g.loadDun(name), t1Floor, t1Dirt, true)
	g.l1FillFloor()
	g.savePdungeon()
}

// loadL1 ports LoadL1Dungeon.
func (g *gen) loadL1(name string, vx, vy int) {
	g.initTrans()
	d := g.loadDun(name)
	g.loadSetTiles(d, t1Floor, t1Dirt, true)
	g.l1FillFloor()
	g.view = point{vx, vy}
	g.lPass3(t1Dirt - 1)
	g.clearGlobals()
	g.setMapMonsters(d, 0, 0)
	g.setMapObjects(d, 0, 0)
}

// loadPreL2 ports LoadPreL2Dungeon.
func (g *gen) loadPreL2(name string) {
	g.initTrans()
	g.loadSetTiles(g.loadDun(name), 3, 12, true)
	g.savePdungeon()
}

// loadL2 ports LoadL2Dungeon.
func (g *gen) loadL2(name string, vx, vy int) {
	g.initTrans()
	d := g.loadDun(name)
	g.loadSetTiles(d, 3, 12, true)
	g.lPass3(12 - 1)
	g.clearGlobals()
	g.view = point{vx, vy}
	g.setMapMonsters(d, 0, 0)
	g.setMapObjects(d, 0, 0)
}

// loadL3 ports LoadPreL3Dungeon and LoadL3Dungeon (Poisoned Water loads the same map for both).
func (g *gen) loadL3(name string) {
	g.initTrans()
	d := g.loadDun(name)
	g.loadSetTiles(d, 7, 8, false)
	g.savePdungeon()
	g.initTrans()
	g.loadSetTiles(d, 7, 8, false)
	g.lPass3(8 - 1)
	g.clearGlobals()
	g.view = point{31, 83}
	g.setMapMonsters(d, 0, 0)
	g.setMapObjects(d, 0, 0)
}

// clearGlobals ports DRLG_Init_Globals' reset of the placement maps.
func (g *gen) clearGlobals() {
	g.flags = [112][112]byte{}
	g.dMonster = [112][112]int{}
	g.dObject = [112][112]int8{}
	g.dItem = [112][112]int8{}
}

// objRange sets the megatile range and lever id of the set level object at (x, y) (SetObjMapRange with ObjIndex).
func (g *gen) objRange(x, y, x1, y1, x2, y2, v int) {
	if o := g.objAt(x, y); o != nil {
		o.Var1, o.Var2, o.Var3, o.Var4, o.Var8 = x1, y1, x2, y2, v
	}
}

// loadSetMap ports LoadSetMap.
func (g *gen) loadSetMap(sl int, seeds [17]int32) {
	g.setlevel = true
	switch sl {
	case SLSkelKing:
		if q := &g.quests[QSkelKing]; q.Active == QuestInit {
			q.Active, q.Var1 = QuestActive, 1
		}
		g.loadPreL1("l1/sklkng1")
		g.loadL1("l1/sklkng2", 83, 45)
		g.areaTrans(skelKingTrans1)
		g.listTrans(skelKingTrans2)
		g.areaTrans(skelKingTrans3)
		g.listTrans(skelKingTrans4)
		g.addLevelObjs(DTypeCathedral)
		g.objRange(64, 34, 20, 7, 23, 10, 1)
		g.objRange(64, 59, 20, 14, 21, 16, 2)
		g.objRange(27, 37, 8, 1, 15, 11, 3)
		g.objRange(46, 35, 8, 1, 15, 11, 3)
		g.objRange(49, 53, 8, 1, 15, 11, 3)
		g.objRange(27, 53, 8, 1, 15, 11, 3)
		g.trigs = []Trigger{{82, 42, TrigReturnLevel}}
	case SLBoneChamb:
		g.loadPreL2("l2/bonecha2")
		g.loadL2("l2/bonecha1", 69, 39)
		g.listTrans(skelChamTrans1)
		g.areaTrans(skelChamTrans2)
		g.listTrans(skelChamTrans3)
		g.addLevelObjs(DTypeCatacombs)
		g.objRange(37, 30, 17, 0, 21, 5, 1)
		g.objRange(37, 46, 13, 0, 16, 5, 2)
		g.trigs = []Trigger{{70, 39, TrigReturnLevel}}
	case SLPoisonWater:
		if q := &g.quests[QPWater]; q.Active == QuestInit {
			q.Active = QuestActive
		}
		g.loadL3("l3/foulwatr")
		g.trigs = []Trigger{{30, 83, TrigReturnLevel}}
	case SLVileBetrayer:
		if q := &g.quests[QBetrayer]; q.Active == QuestDone {
			q.Var2 = 4
		} else if q.Active == QuestActive {
			q.Var2 = 3
		}
		g.loadPreL1("l1/vile1")
		g.loadL1("l1/vile2", 35, 36)
		g.addLevelObjs(DTypeCathedral)
		g.objRange(26, 45, 1, 1, 9, 10, 1)
		g.objRange(45, 46, 11, 1, 20, 10, 2)
		g.objRange(35, 36, 7, 11, 13, 18, 3)
		g.loadTransparency(g.loadDun("l1/vile1"))
		g.trigs = nil
	}
}
