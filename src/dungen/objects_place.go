package dungen

import (
	"rpg/src/geometry"
	"rpg/src/rules"
)

const (
	maxObjects = 127

	misArrow     = 0
	misFirebolt  = 1
	misLightCtrl = 23
	misFlameC    = 44
)

func (g *gen) clrAllObjects() {
	g.objects = [maxObjects]*rules.Object{}
	g.nobjects = 0
	for i := range g.objectavail {
		g.objectavail[i] = i
		g.objectactive[i] = 0
	}
	g.trapdir, g.trapid, g.leverid = 0, 1, 1
}

func (g *gen) objAt(x, y int) *rules.Object {
	v := int(g.dObj(x, y))
	switch {
	case v > 0:
		return g.objects[v-1]
	case v < 0:
		return g.objects[-(v + 1)]
	}
	return nil
}

func (g *gen) setDObject(x, y, v int) {
	if x, y, ok := flat(x, y); ok {
		g.dObject[x][y] = int8(v)
	}
}

// rndLocOk ports RndLocOk.
func (g *gen) rndLocOk(x, y int) bool {
	return g.dMon(x, y) == 0 && g.dObj(x, y) == 0 && g.flagAt(x, y)&bflagPopulated == 0 && !g.solid(x, y)
}

func (g *gen) initRndLocObj(min, max, kind int) {
	for n := g.rnd(max-min) + min; n > 0; n-- {
		for {
			x, y := g.rnd(80)+16, g.rnd(80)+16
			if g.areaOk(x, y, -1, -1, 1, 1) {
				g.addObject(kind, x, y)
				break
			}
		}
	}
}

func (g *gen) areaOk(x, y, x1, y1, x2, y2 int) bool {
	for dy := y1; dy <= y2; dy++ {
		for dx := x1; dx <= x2; dx++ {
			if !g.rndLocOk(x+dx, y+dy) {
				return false
			}
		}
	}
	return true
}

func (g *gen) initRndLocBigObj(min, max, kind int) {
	for n := g.rnd(max-min) + min; n > 0; n-- {
		for {
			x, y := g.rnd(80)+16, g.rnd(80)+16
			if g.areaOk(x, y, -1, -2, 1, 1) {
				g.addObject(kind, x, y)
				break
			}
		}
	}
}

// findLoc rolls random spots until an area around one is free, giving up after 20000 tries (AddBookLever,
// InitRndLocObj5x5, AddStoryBooks).
func (g *gen) findLoc(x1, y1, x2, y2, maxTries int) (int, int, bool) {
	for cnt := 0; ; {
		x, y := g.rnd(80)+16, g.rnd(80)+16
		ok := true
		for n := y1; n <= y2; n++ {
			for m := x1; m <= x2; m++ {
				if !g.rndLocOk(x+m, y+n) {
					ok = false
				}
			}
		}
		if ok {
			return x, y, true
		}
		if cnt++; cnt > maxTries {
			return 0, 0, false
		}
	}
}

func (g *gen) initRndLocObj5x5(min, max, kind int) {
	for n := min + g.rnd(max-min); n > 0; n-- {
		x, y, ok := g.findLoc(-2, -2, 2, 2, 20000)
		if !ok {
			return
		}
		g.addObject(kind, x, y)
	}
}

// addBookLever ports AddBookLever: the book that opens a quest set piece (x1..y2 in megatiles).
func (g *gen) addBookLever(x1, y1, x2, y2, msg int) {
	xp, yp, ok := g.findLoc(-2, -2, 2, 2, 20000)
	if !ok {
		return
	}
	if g.available(rules.QBlind) {
		g.addObject(rules.ObjBLINDBOOK, xp, yp)
	}
	if g.available(rules.QWarlord) {
		g.addObject(rules.ObjSTEELTOME, xp, yp)
	}
	if g.available(rules.QBlood) { // the book is its set piece's own
		if x, y, ok := g.objectOfKind(rules.ObjBLOODBOOK); ok {
			xp, yp = x, y
		}
	}
	o := g.objAt(xp, yp)
	if o == nil {
		return
	}
	o.Var1, o.Var2, o.Var3, o.Var4, o.Var8 = x1, y1, x2, y2, g.leverid
	o.Var7 = msg
	g.leverid++
	o.Var6 = o.AnimFrame + 1
}

func (g *gen) initRndBarrels() {
	bxadd := [8]int{-1, 0, 1, -1, 1, -1, 0, 1}
	byadd := [8]int{-1, -1, -1, 0, 0, 1, 1, 1}
	for n := g.rnd(5) + 3; n > 0; n-- {
		var xp, yp int
		for {
			xp, yp = g.rnd(80)+16, g.rnd(80)+16
			if g.rndLocOk(xp, yp) {
				break
			}
		}
		kind := rules.ObjBARRELEX
		if g.rnd(4) != 0 {
			kind = rules.ObjBARREL
		}
		g.addObject(kind, xp, yp)
		found, p, c := true, 0, 1
		for g.rnd(p) == 0 && found {
			found = false
			for t := 0; t < 3; t++ {
				d := g.rnd(8)
				xp += bxadd[d]
				yp += byadd[d]
				if found = g.rndLocOk(xp, yp); found {
					break
				}
			}
			if found {
				kind := rules.ObjBARRELEX
				if g.rnd(5) != 0 {
					kind = rules.ObjBARREL
				}
				g.addObject(kind, xp, yp)
				c++
			}
			p = c >> 1
		}
	}
}

func (g *gen) wallTrapLocOk(x, y int) bool {
	return g.flagAt(x, y)&bflagPopulated == 0 && g.tileFlags(x, y).HoldsTrap &&
		objKindOrNone(g.objAt(x-1, y)) != rules.ObjTORCHL && objKindOrNone(g.objAt(x, y-1)) != rules.ObjTORCHR // hangTorchesOnWalls' wall
}

// addObjTraps ports AddObjTraps: arrow, fire and lightning traps in the walls aimed at doors and chests.
func (g *gen) addObjTraps() {
	var rndv int
	switch {
	case g.currlevel >= 7:
		rndv = 25
	case g.currlevel >= 5:
		rndv = 20
	case g.currlevel >= 2:
		rndv = 15
	case g.currlevel == 1:
		rndv = 10
	}
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.dObject[i][j] <= 0 || g.rnd(100) >= rndv {
				continue
			}
			o := g.objects[g.dObject[i][j]-1]
			if !g.data.Objects[o.Kind].Trap {
				continue
			}
			if g.rnd(2) == 0 {
				xp := i - 1
				for xp >= 0 && !g.solid(xp, j) {
					xp--
				}
				if !g.wallTrapLocOk(xp, j) || i-xp <= 1 {
					continue
				}
				g.addObject(rules.ObjTRAPL, xp, j)
				if t := g.objAt(xp, j); t != nil {
					t.Var1, t.Var2 = i, j
				}
			} else {
				yp := j - 1
				for yp >= 0 && !g.solid(i, yp) {
					yp--
				}
				if !g.wallTrapLocOk(i, yp) || j-yp <= 1 {
					continue
				}
				g.addObject(rules.ObjTRAPR, i, yp)
				if t := g.objAt(i, yp); t != nil {
					t.Var1, t.Var2 = i, j
				}
			}
			o.TrapFlag = true
		}
	}
}

func (g *gen) addChestTraps() {
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.dObject[i][j] <= 0 {
				continue
			}
			o := g.objects[g.dObject[i][j]-1]
			if o.Kind >= rules.ObjCHEST1 && o.Kind <= rules.ObjCHEST3 && !o.TrapFlag && g.rnd(100) < 10 {
				o.Kind += rules.ObjTCHEST1 - rules.ObjCHEST1
				o.TrapFlag = true
				if g.ltype == rules.DTypeCatacombs {
					o.Var4 = g.rnd(2)
				} else {
					o.Var4 = g.rnd(3)
				}
			}
		}
	}
}

func (g *gen) addStoryBooks(seeds [rules.NumLevels]int32) {
	xp, yp, ok := g.findLoc(-3, -2, 3, 2, 20000)
	if !ok {
		return
	}
	g.addStoryBook(xp, yp, seeds)
	for _, c := range [][2]int{{-2, 1}, {-2, 0}, {-1, -1}, {1, -1}, {2, 0}, {2, 1}} {
		g.addObject(rules.ObjSTORYCANDLE, xp+c[0], yp+c[1])
	}
}

func (g *gen) addL4Goodies() {
	g.addCellHookedBodies(6)
	for _, k := range []int{rules.ObjTNUDEM1, rules.ObjTNUDEM2, rules.ObjTNUDEM3, rules.ObjTNUDEM4, rules.ObjTNUDEW1, rules.ObjTNUDEW2, rules.ObjTNUDEW3, rules.ObjDECAP} {
		g.initRndLocObj(2, 6, k)
	}
	g.initRndLocObj(1, 3, rules.ObjCAULDRON)
}

// addLazStand ports AddLazStand: the Staff of Lazarus on its stand, with candles and bodies around.
func (g *gen) addLazStand() {
	xp, yp, ok := g.findLoc(-2, -3, 3, 3, 10000)
	if !ok {
		g.initRndLocObj(1, 1, rules.ObjLAZSTAND)
		return
	}
	for _, o := range [...][3]int{{rules.ObjLAZSTAND, 0, 0}, {rules.ObjTNUDEM2, 0, 2}, {rules.ObjSTORYCANDLE, 1, 2}, {rules.ObjTNUDEM3, 2, 2},
		{rules.ObjTNUDEW1, 0, -2}, {rules.ObjSTORYCANDLE, 1, -2}, {rules.ObjTNUDEW2, 2, -2}, {rules.ObjSTORYCANDLE, -1, -1}, {rules.ObjTNUDEW3, -1, 0},
		{rules.ObjSTORYCANDLE, -1, 1}} {
		g.addObject(o[0], xp+o[1], yp+o[2])
	}
}

// getRndObjLoc ports GetRndObjLoc (random area of randarea×randarea free cells, anywhere on the map).
func (g *gen) getRndObjLoc(randarea int) (int, int) {
	for tries := 0; ; {
		if tries++; tries > 1000 && randarea > 1 {
			randarea--
		}
		xx, yy := g.rnd(112), g.rnd(112)
		failed := false
		for i := 0; i < randarea && !failed; i++ {
			for j := 0; j < randarea && !failed; j++ {
				failed = !g.rndLocOk(i+xx, j+yy)
			}
		}
		if !failed {
			return xx, yy
		}
	}
}

// initObjects ports InitObjects.
func (g *gen) initObjects(seeds [rules.NumLevels]int32) {
	g.clrAllObjects()
	if g.currlevel == 16 {
		g.pieceObjects()
		return
	}
	g.initObjFlag = true
	g.rng.Seed()
	if g.currlevel == 9 {
		x, y := g.getRndObjLoc(5)
		g.addObject(rules.ObjSLAINHERO, x+2, y+2)
	}
	if g.currlevel == g.quests[rules.QMushroom].Level && g.quests[rules.QMushroom].Active == rules.QuestInit && g.nobjects < maxObjects {
		x, y := g.getRndObjLoc(5)
		oi := g.objectavail[0]
		extra := [][2]int{{x + 1, y + 1}, {x + 2, y + 1}, {x + 1, y + 2}}
		for _, c := range extra {
			g.setDObject(c[0], c[1], -1-oi)
		}
		g.addObject(rules.ObjMUSHPATCH, x+2, y+2)
		if o := g.objAt(x+2, y+2); o != nil {
			o.ExtraCells = extra
		}
	}
	if g.currlevel == 4 || g.currlevel == 8 || g.currlevel == 12 {
		g.addStoryBooks(seeds)
	}
	sp := g.setPiece
	g.pieceObjects() // the set pieces' own: the Butcher's tortures, the candles, the sign chest, the levers
	switch g.ltype {
	case rules.DTypeCathedral:
		g.initRndLocBigObj(10, 15, rules.ObjSARC)
		g.addCellObjs(rules.DTypeCathedral)
		g.initRndBarrels()
	case rules.DTypeCatacombs:
		if g.available(rules.QRock) {
			g.initRndLocObj5x5(1, 1, rules.ObjSTAND)
		}
		if g.available(rules.QSchamb) {
			g.initRndLocObj5x5(1, 1, rules.ObjBOOK2R)
		}
		g.addCellObjs(rules.DTypeCatacombs)
		g.addCellTorches()
		if g.available(rules.QBlind) {
			g.addBookLever(sp.x, sp.y, sp.w+sp.x+1, sp.h+sp.y+1, 0)
		}
		if g.available(rules.QBlood) {
			g.addBookLever(sp.x, sp.y+3, sp.x+2, sp.y+7, 0)
		}
		g.initRndBarrels()
	case rules.DTypeCaves:
		g.addCellObjs(rules.DTypeCaves)
		g.initRndBarrels()
	case rules.DTypeHell:
		if g.available(rules.QWarlord) {
			g.addBookLever(sp.x, sp.y, sp.x+sp.w, sp.y+sp.h, 0)
		}
		if g.available(rules.QBetrayer) {
			g.addLazStand()
		}
		g.initRndBarrels()
		g.addL4Goodies()
	}
	g.initRndLocObj(5, 10, rules.ObjCHEST1)
	g.initRndLocObj(3, 6, rules.ObjCHEST2)
	g.initRndLocObj(1, 5, rules.ObjCHEST3)
	if g.ltype != rules.DTypeHell {
		g.addObjTraps()
	}
	if g.ltype > rules.DTypeCathedral {
		g.addChestTraps()
	}
	g.initObjFlag = false
}

// addObject ports AddObject and the setup of each kind.
func (g *gen) addObject(kind, ox, oy int) {
	if g.nobjects >= maxObjects || kind < 0 {
		return
	}
	oi := g.objectavail[0]
	g.objectavail[0] = g.objectavail[maxObjects-1-g.nobjects]
	g.objectactive[g.nobjects] = oi
	g.setDObject(ox, oy, oi+1)
	d := &g.data.Objects[kind]
	o := &rules.Object{Kind: kind, X: ox, Y: oy, Index: oi, Solid: d.Solid, Missile: d.Missile, SelFlag: d.SelFlag, Active: true}
	g.objects[oi] = o
	if d.AnimFlag != 0 {
		g.rnd(d.AnimDelay) // _oAnimCnt
		o.AnimFrame = g.rnd(d.AnimLen-1) + 1
	} else {
		o.AnimFrame = d.AnimDelay
	}
	switch kind {
	case rules.ObjL1LDOOR, rules.ObjL1RDOOR, rules.ObjL2LDOOR, rules.ObjL2RDOOR, rules.ObjL3LDOOR, rules.ObjL3RDOOR:
		o.DoorFlag = true
	case rules.ObjBOOK2R:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.setPiece.x, g.setPiece.y, g.setPiece.w+g.setPiece.x+1, g.setPiece.h+g.setPiece.y+1
		o.Var6 = o.AnimFrame + 1
	case rules.ObjCHEST1, rules.ObjCHEST2, rules.ObjCHEST3, rules.ObjTCHEST1, rules.ObjTCHEST2, rules.ObjTCHEST3:
		if g.rnd(2) == 0 {
			o.AnimFrame += 3
		}
		o.RndSeed = g.rng.Seed()
		n := map[int]int{rules.ObjCHEST1: 1, rules.ObjTCHEST1: 1, rules.ObjCHEST2: 2, rules.ObjTCHEST2: 2, rules.ObjCHEST3: 3, rules.ObjTCHEST3: 3}[kind]
		if g.setlevel {
			o.Var1 = n
		} else {
			o.Var1 = g.rnd(n + 1)
		}
		o.Var2 = g.rnd(8)
		o.Var4 = 0
	case rules.ObjSARC:
		g.setDObject(ox, oy-1, -(oi + 1))
		o.ExtraCells = [][2]int{{ox, oy - 1}}
		o.Var1 = g.rnd(10)
		o.RndSeed = g.rng.Seed()
		if o.Var1 >= 8 {
			o.Var2 = g.preSpawnSkeleton()
		}
	case rules.ObjFLAMEHOLE:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.trapid, 0, g.trapdir, 0
	case rules.ObjFLAMELVR:
		o.Var1, o.Var2 = g.trapid, misFlameC
	case rules.ObjWATER:
		o.AnimFrame = 1
	case rules.ObjTRAPL, rules.ObjTRAPR:
		mt := g.currlevel/3 + 1
		switch g.rnd(mt) {
		case 0:
			o.Var3 = misArrow
		case 1:
			o.Var3 = misFirebolt
		case 2:
			o.Var3 = misLightCtrl
		}
		o.Var4 = 0
	case rules.ObjBARREL, rules.ObjBARRELEX:
		o.Var1 = 0
		o.RndSeed = g.rng.Seed()
		o.Var2 = g.rnd(10)
		o.Var3 = g.rnd(3)
		if o.Var2 >= 8 {
			o.Var4 = g.preSpawnSkeleton()
		}
	case rules.ObjSHRINEL, rules.ObjSHRINER:
		o.PreFlag = true
		for {
			v := g.rnd(len(g.data.ShrineNames))
			if g.currlevel <= g.data.ShrineMax[v] && g.data.ShrineAvail[v] != 2 {
				o.Var1 = v
				break
			}
		}
		if g.rnd(2) != 0 {
			o.AnimFrame = 12
		}
	case rules.ObjBOOKCASEL, rules.ObjBOOKCASER:
		o.RndSeed = g.rng.Seed()
		o.PreFlag = true
	case rules.ObjSKELBOOK, rules.ObjBOOKSTAND, rules.ObjBLOODFTN, rules.ObjGOATSHRINE, rules.ObjCAULDRON, rules.ObjTEARFTN:
		o.RndSeed = g.rng.Seed()
	case rules.ObjDECAP:
		o.RndSeed = g.rng.Seed()
		o.AnimFrame = g.rnd(8) + 1
		o.PreFlag = true
	case rules.ObjPURIFYINGFTN, rules.ObjMURKYFTN:
		o.ExtraCells = [][2]int{{ox, oy - 1}, {ox - 1, oy}, {ox - 1, oy - 1}}
		for _, c := range o.ExtraCells {
			g.setDObject(c[0], c[1], -1-oi)
		}
		o.RndSeed = g.rng.Seed()
	case rules.ObjARMORSTAND, rules.ObjWARARMOR:
		if !g.armorFlag {
			o.AnimDisabled, o.SelFlag = true, 0
		}
		o.RndSeed = g.rng.Seed()
	case rules.ObjBOOK2L:
		if g.setlevel && g.setLvlNum == rules.SLVileBetrayer {
			o.AnimFrame = 4
		}
	case rules.ObjMCIRCLE1, rules.ObjMCIRCLE2:
		o.RndSeed = g.rng.Seed()
		o.PreFlag = true
		o.Var6, o.Var5 = 0, 1
	case rules.ObjBCROSS, rules.ObjTBCROSS:
		o.RndSeed = g.rng.Seed()
	case rules.ObjPEDISTAL:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.setPiece.x, g.setPiece.y, g.setPiece.x+g.setPiece.w, g.setPiece.y+g.setPiece.h
	case rules.ObjWARWEAP, rules.ObjWEAPONRACK:
		if !g.weaponFlag {
			o.AnimDisabled, o.SelFlag = true, 0
		}
		o.RndSeed = g.rng.Seed()
	case rules.ObjTNUDEM2:
		o.RndSeed = g.rng.Seed()
		o.AnimFrame = g.rnd(4) + 1
		o.PreFlag = true
	}
	g.nobjects++
}

// addStoryBook ports AddObject(OBJ_STORYBOOK) with AddStoryBook, which reseeds the RNG from level 16's seed.
func (g *gen) addStoryBook(x, y int, seeds [rules.NumLevels]int32) {
	g.addObject(rules.ObjSTORYBOOK, x, y)
	o := g.objAt(x, y)
	if o == nil {
		return
	}
	g.rng.SetSeed(seeds[16])
	o.Var1 = g.rnd(3)                      // which of the three narrations (StoryText row)
	o.Var2 = g.currlevel/4 - 1             // StoryText column: the Cathedral, Catacombs or Caves book
	o.Var3 = g.currlevel>>2 + 3*o.Var1 - 1 // StoryBookName index
	o.AnimFrame = 5 - 2*o.Var1
	o.Var4 = o.AnimFrame + 1
}

// preSpawnSkeleton ports PreSpawnSkeleton: a skeleton kept off the map for a sarcophagus or barrel.
func (g *gen) preSpawnSkeleton() int {
	j := 0
	for _, t := range g.mtypes {
		if isSkel(t.mtype) {
			j++
		}
	}
	if j == 0 {
		return -1
	}
	skeltypes := g.rnd(j)
	i := 0
	for j = 0; i < len(g.mtypes) && j <= skeltypes; i++ {
		if isSkel(g.mtypes[i].mtype) {
			j++
		}
	}
	slot := g.addMonster(0, 0, 0, i-1, false)
	if slot >= 0 {
		g.monsters[slot].Hidden = true
	}
	return slot
}

// moveMonstersOffObjects keeps monsters and solid objects off the same cell. D1 places monsters before the
// layout's doors, so a monster can end up standing inside one; objects win (the door's cell is the wall's), so
// the monster is nudged to the first free neighbour instead. Done after placement, with no RNG of its own, so
// a level built from a seed still matches D1 everywhere else.
func (g *gen) moveMonstersOffObjects() {
	for _, m := range g.monsters {
		o := g.objectAt(m.X, m.Y)
		if o == nil || !o.Solid && !o.DoorFlag {
			continue
		}
		for _, dir := range geometry.Dirs {
			x, y := m.X+dir.X, m.Y+dir.Y
			if !g.monstPlace(x, y) || g.objectAt(x, y) != nil {
				continue
			}
			slot := g.dMon(m.X, m.Y)
			if fx, fy, ok := flat(m.X, m.Y); ok {
				g.dMonster[fx][fy] = 0
			}
			if fx, fy, ok := flat(x, y); ok {
				g.dMonster[fx][fy] = slot
			}
			m.X, m.Y = x, y
			break
		}
	}
}

// objectAt is the object standing on a cell, or nil.
func (g *gen) objectAt(x, y int) *rules.Object {
	if oi := g.dObj(x, y); oi > 0 && int(oi) <= maxObjects {
		return g.objects[oi-1]
	}
	return nil
}

// hangTorchesOnWalls moves the two wall torches AddL2Torches hangs off a wall onto the wall itself. D1 puts
// them on the floor cell in front, where its 3/4 view still draws them against the wall behind; top-down that
// leaves a torch standing in the open. Run after every object is placed, so nothing else shifts.
func (g *gen) hangTorchesOnWalls() {
	back := map[int][2]int{rules.ObjTORCHL: {1, 0}, rules.ObjTORCHR: {0, 1}} // the cell AddL2Torches stepped off
	for _, o := range g.objects {
		d, ok := back[objKindOrNone(o)]
		if !ok {
			continue
		}
		x, y := o.X+d[0], o.Y+d[1]
		if !g.solidLoc(x, y) || g.dObj(x, y) != 0 {
			continue
		}
		g.setDObject(o.X, o.Y, 0)
		g.setDObject(x, y, o.Index+1)
		o.X, o.Y = x, y
	}
}

func objKindOrNone(o *rules.Object) int {
	if o == nil {
		return -1
	}
	return o.Kind
}

// objectOfKind is where the first object of that kind stands.
func (g *gen) objectOfKind(kind int) (int, int, bool) {
	for _, oi := range g.objectactive[:g.nobjects] {
		if o := g.objects[oi]; o.Kind == kind {
			return o.X, o.Y, true
		}
	}
	return 0, 0, false
}
