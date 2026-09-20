package d1

import (
	"fmt"
)

// objects.cpp: objects placed on level creation.

const (
	maxObjects = 127

	misArrow     = 0
	misFirebolt  = 1
	misLightCtrl = 23
	misFlameC    = 44
)

// ObjData is a row of objdat.cpp's AllObjects that matters after placement.
type ObjData struct {
	Name                                 string
	AnimFlag, AnimDelay, AnimLen         int
	Solid, Missile, Light, Trap          bool
	Break, SelFlag                       int
	MinLvl, MaxLvl, LvlType, Theme, Load int
}

// Object is a placed D1 object (ObjectStruct minus graphics), in piece coordinates.
type Object struct {
	Kind         int // Obj* (row of Data.Objects)
	X, Y         int
	Index        int // D1's object slot; dObject holds Index+1
	AnimFrame    int
	SelFlag      int
	Solid        bool
	Missile      bool
	TrapFlag     bool
	DoorFlag     bool
	PreFlag      bool
	RndSeed      int32
	Var1, Var2   int // doors: pieces; chests: item count, item kind; shrines: type; barrels/sarcophagi: contents
	Var3, Var4   int // traps: missile; barrels/sarcophagi: pre-spawned skeleton slot (-1 none)
	Var5, Var6   int
	Var7, Var8   int      // books: message, lever id
	ExtraCells   [][2]int // cells D1 marks with -(Index+1): sarcophagi, fountains, mushroom patch
	AnimDisabled bool     // _oAnimFlag 2: an empty armor stand or weapon rack
	Active       bool     // game state (not part of D1's placement): true until opened, broken or pulled
}

// ShrineNames are D1's shrinestrs.
var ShrineNames = [34]string{"Mysterious", "Hidden", "Gloomy", "Weird", "Magical", "Stone", "Religious", "Enchanted",
	"Thaumaturgic", "Fascinating", "Cryptic", "Magical", "Eldritch", "Eerie", "Divine", "Holy", "Sacred", "Spiritual",
	"Spooky", "Abandoned", "Creepy", "Quiet", "Secluded", "Ornate", "Glimmering", "Tainted",
	"Oily", "Glowing", "Mendicant's", "Sparkling", "Town", "Shimmering", "Solar", "Murphy's"} // Hellfire's last eight

// ShrineMessages are the MsgStrings each shrine shows, with a note on what the shrine does.
var ShrineMessages = [34]string{"Some are weakened as one grows strong (-1 all stats, +6 to one random stat permanently)",
	"New strength is forged through destruction (+10 max durability on equipped items, one random item -20)",
	"Those who defend seldom attack (+2 armor on all armor, -1 max damage on all weapons)",
	"The sword of justice is swift and sharp (+1 max damage on all weapons)",
	"While the spirit is vigilant the body thrives (Mana Shield)",
	"The powers of mana refocused renews (recharges all staves)",
	"Time cannot diminish the power of steel (repairs all items)",
	"Magic is not always what it seems to be (+1 level to known spells, -2 to one random spell)",
	"What once was opened now is closed (refills all chests on this level)",
	"Intensity comes at the cost of wisdom (+2 Firebolt levels, -10% max mana permanently)",
	"Arcane power brings destruction (casts Nova, restores mana)",
	"While the spirit is vigilant the body thrives (Mana Shield)",
	"Crimson and Azure become as the sun (healing and mana potions in pack become rejuvenation)",
	"Knowledge and wisdom at the cost of self (+2 Mag permanently)",
	"Drink and be refreshed (full life and mana, drops 2 potions)",
	"Wherever you go, there you are (teleports you to a random spot)",
	"Energy comes at the cost of wisdom (+2 Charged Bolt levels, -10% max mana permanently)",
	"Riches abound when least expected (fills empty inventory slots with gold)",
	"Where avarice fails, patience gains reward (no effect in single player)",
	"The hands of men may be guided by fate (+2 Dex permanently)",
	"Strength is bolstered by heavenly faith (+2 Str permanently)",
	"The essence of life flows from within (+2 Vit permanently)",
	"The way is made clear when viewed from above (reveals the level map)",
	"Salvation comes at the cost of wisdom (+2 Holy Bolt levels, -10% max mana permanently)",
	"Mysteries are revealed in the light of reason (identifies all items)",
	"Those who are last may yet be first (no effect in single player)",
	"That which does not kill you... (+2 to your class' main stat, sets off a fire wall)",
	"Knowledge is power (+1 Mag per 1000 XP, up to +5 for 5% of your XP)",
	"Give and you shall receive (half your gold becomes XP)",
	"Some experience is gained by touch (1000 XP per dungeon level, sets off a flash)",
	"There's no place like home (opens a town portal)",
	"Spirtual energy is restored (full mana)",
	"The sun's position decides (+2 Dex, Str, Mag or Vit by the time of day)",
	"That which can break will (halves an equipped item's durability, or takes a third of your gold)"}

// StoryBookNames are D1's StoryBookName.
var StoryBookNames = [9]string{"The Great Conflict", "The Wages of Sin are War", "The Tale of the Horadrim", "The Dark Exile",
	"The Sin War", "The Binding of the Three", "The Realms Beyond", "Tale of the Three", "The Black King"}

var (
	shrineMax   = [34]int{24, 24, 24, 24, 24, 24, 24, 8, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24, 24}
	shrineAvail = [34]int{0, 0, 1, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 1, 0} // 2 = multiplayer only
)

func (g *gen) clrAllObjects() {
	g.objects = [maxObjects]*Object{}
	g.nobjects = 0
	for i := range g.objectavail {
		g.objectavail[i] = i
		g.objectactive[i] = 0
	}
	g.trapdir, g.trapid, g.leverid = 0, 1, 1
}

func (g *gen) objAt(x, y int) *Object {
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
	if g.dMon(x, y) != 0 || g.dObj(x, y) != 0 || g.flagAt(x, y)&bflagPopulated != 0 || g.solid(x, y) {
		return false
	}
	p := g.pieceAt(x, y) + 1
	return g.ltype != DTypeCathedral || p <= 126 || p >= 144
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

func (g *gen) addTortures() {
	for oy := 0; oy < 112; oy++ {
		for ox := 0; ox < 112; ox++ {
			if g.piece[ox][oy]+1 != 367 {
				continue
			}
			for _, o := range [...][3]int{{ObjTORTURE1, 0, 1}, {ObjTORTURE3, 2, -1}, {ObjTORTURE2, 0, 3}, {ObjTORTURE4, 4, -1},
				{ObjTORTURE5, 0, 5}, {ObjTNUDEM1, 1, 3}, {ObjTNUDEM2, 4, 5}, {ObjTNUDEM3, 2, 0}, {ObjTNUDEM4, 3, 2},
				{ObjTNUDEW1, 2, 4}, {ObjTNUDEW2, 2, 1}, {ObjTNUDEW3, 4, 2}} {
				g.addObject(o[0], ox+o[1], oy+o[2])
			}
		}
	}
}

func (g *gen) addCandles() {
	tx, ty := g.quests[QPWater].X, g.quests[QPWater].Y
	g.addObject(ObjSTORYCANDLE, tx-2, ty+1)
	g.addObject(ObjSTORYCANDLE, tx+3, ty+1)
	g.addObject(ObjSTORYCANDLE, tx-1, ty+2)
	g.addObject(ObjSTORYCANDLE, tx+2, ty+2)
}

// addBookLever ports AddBookLever: the book that opens a quest set piece (x1..y2 in megatiles).
func (g *gen) addBookLever(x1, y1, x2, y2, msg int) {
	xp, yp, ok := g.findLoc(-2, -2, 2, 2, 20000)
	if !ok {
		return
	}
	if g.available(QBlind) {
		g.addObject(ObjBLINDBOOK, xp, yp)
	}
	if g.available(QWarlord) {
		g.addObject(ObjSTEELTOME, xp, yp)
	}
	if g.available(QBlood) {
		xp, yp = 2*g.setPiece.x+25, 2*g.setPiece.y+40
		g.addObject(ObjBLOODBOOK, xp, yp)
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
		kind := ObjBARRELEX
		if g.rnd(4) != 0 {
			kind = ObjBARREL
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
				kind := ObjBARRELEX
				if g.rnd(5) != 0 {
					kind = ObjBARREL
				}
				g.addObject(kind, xp, yp)
				c++
			}
			p = c >> 1
		}
	}
}

// addLevelObjs ports AddL1Objs/AddL2Objs/AddL3Objs: lights and doors on their pieces.
func (g *gen) addLevelObjs(ltype int) {
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			pn := g.piece[i][j] + 1
			if ltype == DTypeCathedral && pn == 270 {
				g.addObject(ObjL1LIGHT, i, j)
			}
			if k := doorKind(ltype, pn); k >= 0 {
				g.addObject(k, i, j)
			}
		}
	}
}

// doorKind is the door object AddL1Objs/AddL2Objs/AddL3Objs put on D1 piece pn, or -1.
func doorKind(ltype, pn int) int {
	switch {
	case ltype == DTypeCathedral && (pn == 44 || pn == 51 || pn == 214):
		return ObjL1LDOOR
	case ltype == DTypeCathedral && (pn == 46 || pn == 56):
		return ObjL1RDOOR
	case ltype == DTypeCatacombs && (pn == 13 || pn == 541):
		return ObjL2LDOOR
	case ltype == DTypeCatacombs && (pn == 17 || pn == 542):
		return ObjL2RDOOR
	case ltype == DTypeCaves && pn == 531:
		return ObjL3LDOOR
	case ltype == DTypeCaves && pn == 534:
		return ObjL3RDOOR
	}
	return -1
}

func (g *gen) addL2Torches() {
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if g.flags[i][j]&bflagPopulated != 0 {
				continue
			}
			pn := g.piece[i][j] + 1
			if pn == 1 && g.rnd(3) == 0 {
				g.addObject(ObjTORCHL2, i, j)
			}
			if pn == 5 && g.rnd(3) == 0 {
				g.addObject(ObjTORCHR2, i, j)
			}
			if pn == 37 && g.rnd(10) == 0 && g.dObj(i-1, j) == 0 {
				g.addObject(ObjTORCHL, i-1, j)
			}
			if pn == 41 && g.rnd(10) == 0 && g.dObj(i, j-1) == 0 {
				g.addObject(ObjTORCHR, i, j-1)
			}
		}
	}
}

func (g *gen) wallTrapLocOk(x, y int) bool {
	return g.flagAt(x, y)&bflagPopulated == 0 && g.solFlag(x, y, 0x80)
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
				g.addObject(ObjTRAPL, xp, j)
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
				g.addObject(ObjTRAPR, i, yp)
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
			if o.Kind >= ObjCHEST1 && o.Kind <= ObjCHEST3 && !o.TrapFlag && g.rnd(100) < 10 {
				o.Kind += ObjTCHEST1 - ObjCHEST1
				o.TrapFlag = true
				if g.ltype == DTypeCatacombs {
					o.Var4 = g.rnd(2)
				} else {
					o.Var4 = g.rnd(6)
				}
			}
		}
	}
}

// loadMapObjects ports LoadMapObjects: a .DUN's objects, each with the megatile range (x1, y1, w, h) and lever id
// it controls.
func (g *gen) loadMapObjects(d dun, startx, starty, x1, y1, w, h, leveridx int) {
	g.initObjFlag = true
	for j := 0; j < 2*d.h(); j++ {
		for i := 0; i < 2*d.w(); i++ {
			if v := d.layer(2, i, j); v != 0 {
				x, y := startx+16+i, starty+16+j
				g.addObject(objTypeConv[v], x, y)
				if o := g.objAt(x, y); o != nil {
					o.Var1, o.Var2, o.Var3, o.Var4, o.Var8 = x1, y1, x1+w, y1+h, leveridx
				}
			}
		}
	}
	g.initObjFlag = false
}

// DunObjects lists the objects a set piece file places when D1 loads it into a built level (LoadMapObjs without
// AddObject's bookkeeping), at piece coordinates from megatile (startx, starty) doubled, as D1 passes them.
func (data *Data) DunObjects(name string, startx, starty int) []*Object {
	d, ok := data.levels.dun[name]
	if !ok {
		panic(fmt.Errorf("D1 level data: no set piece %q", name))
	}
	var objs []*Object
	for j := 0; j < 2*d.h(); j++ {
		for i := 0; i < 2*d.w(); i++ {
			if v := d.layer(2, i, j); v != 0 {
				objs = append(objs, &Object{Kind: objTypeConv[v], X: startx + 16 + i, Y: starty + 16 + j, Index: -1, Active: true})
			}
		}
	}
	return objs
}

// loadMapObjs ports LoadMapObjs.
func (g *gen) loadMapObjs(d dun, startx, starty int) {
	g.initObjFlag = true
	for j := 0; j < 2*d.h(); j++ {
		for i := 0; i < 2*d.w(); i++ {
			if v := d.layer(2, i, j); v != 0 {
				g.addObject(objTypeConv[v], startx+16+i, starty+16+j)
			}
		}
	}
	g.initObjFlag = false
}

func (g *gen) addDiabObjs() {
	q := g.l4.quads
	g.loadMapObjects(g.loadDun("l4/diab1"), 2*q[0].x, 2*q[0].y, q[1].x, q[1].y, 11, 12, 1)
	g.loadMapObjects(g.loadDun("l4/diab2a"), 2*q[1].x, 2*q[1].y, q[2].x, q[2].y, 11, 11, 2)
	g.loadMapObjects(g.loadDun("l4/diab3a"), 2*q[2].x, 2*q[2].y, q[3].x, q[3].y, 9, 9, 3)
}

func (g *gen) addStoryBooks(seeds [NumLevels]int32) {
	xp, yp, ok := g.findLoc(-3, -2, 3, 2, 20000)
	if !ok {
		return
	}
	g.addStoryBook(xp, yp, seeds)
	for _, c := range [][2]int{{-2, 1}, {-2, 0}, {-1, -1}, {1, -1}, {2, 0}, {2, 1}} {
		g.addObject(ObjSTORYCANDLE, xp+c[0], yp+c[1])
	}
}

func (g *gen) addHookedBodies(freq int) {
	for j := 0; j < 40; j++ {
		jj := 16 + 2*j
		for i := 0; i < 40; i++ {
			ii := 16 + 2*i
			if g.d(i, j) != 1 && g.d(i, j) != 2 || g.rnd(freq) != 0 || g.nearThemeRoom(i, j) {
				continue
			}
			if g.d(i, j) == 1 && g.d(i+1, j) == 6 {
				g.addObject([3]int{ObjTORTURE1, ObjTORTURE2, ObjTORTURE5}[g.rnd(3)], ii+1, jj)
				continue
			}
			if g.d(i, j) == 2 && g.d(i, j+1) == 6 {
				g.addObject([2]int{ObjTORTURE3, ObjTORTURE4}[g.rnd(2)], ii, jj)
			}
		}
	}
}

func (g *gen) addL4Goodies() {
	g.addHookedBodies(6)
	for _, k := range []int{ObjTNUDEM1, ObjTNUDEM2, ObjTNUDEM3, ObjTNUDEM4, ObjTNUDEW1, ObjTNUDEW2, ObjTNUDEW3, ObjDECAP} {
		g.initRndLocObj(2, 6, k)
	}
	g.initRndLocObj(1, 3, ObjCAULDRON)
}

// addLazStand ports AddLazStand: the Staff of Lazarus on its stand, with candles and bodies around.
func (g *gen) addLazStand() {
	xp, yp, ok := g.findLoc(-2, -3, 3, 3, 10000)
	if !ok {
		g.initRndLocObj(1, 1, ObjLAZSTAND)
		return
	}
	for _, o := range [...][3]int{{ObjLAZSTAND, 0, 0}, {ObjTNUDEM2, 0, 2}, {ObjSTORYCANDLE, 1, 2}, {ObjTNUDEM3, 2, 2},
		{ObjTNUDEW1, 0, -2}, {ObjSTORYCANDLE, 1, -2}, {ObjTNUDEW2, 2, -2}, {ObjSTORYCANDLE, -1, -1}, {ObjTNUDEW3, -1, 0},
		{ObjSTORYCANDLE, -1, 1}} {
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
func (g *gen) initObjects(seeds [NumLevels]int32) {
	g.clrAllObjects()
	if g.currlevel == 16 {
		g.addDiabObjs()
		return
	}
	g.initObjFlag = true
	g.rng.Seed()
	if g.currlevel == 9 {
		x, y := g.getRndObjLoc(5)
		g.addObject(ObjSLAINHERO, x+2, y+2)
	}
	if g.currlevel == g.quests[QMushroom].Level && g.quests[QMushroom].Active == QuestInit && g.nobjects < maxObjects {
		x, y := g.getRndObjLoc(5)
		oi := g.objectavail[0]
		extra := [][2]int{{x + 1, y + 1}, {x + 2, y + 1}, {x + 1, y + 2}}
		for _, c := range extra {
			g.setDObject(c[0], c[1], -1-oi)
		}
		g.addObject(ObjMUSHPATCH, x+2, y+2)
		if o := g.objAt(x+2, y+2); o != nil {
			o.ExtraCells = extra
		}
	}
	if g.currlevel == 4 || g.currlevel == 8 || g.currlevel == 12 {
		g.addStoryBooks(seeds)
	}
	sp := g.setPiece
	switch g.ltype {
	case DTypeCathedral:
		if g.available(QButcher) {
			g.addTortures()
		}
		if g.available(QPWater) {
			g.addCandles()
		}
		if g.available(QLtBanner) {
			g.addObject(ObjSIGNCHEST, 2*sp.x+26, 2*sp.y+19)
		}
		g.initRndLocBigObj(10, 15, ObjSARC)
		g.addLevelObjs(DTypeCathedral)
		g.initRndBarrels()
	case DTypeCatacombs:
		if g.available(QRock) {
			g.initRndLocObj5x5(1, 1, ObjSTAND)
		}
		if g.available(QSchamb) {
			g.initRndLocObj5x5(1, 1, ObjBOOK2R)
		}
		g.addLevelObjs(DTypeCatacombs)
		g.addL2Torches()
		if g.available(QBlind) {
			g.addBookLever(sp.x, sp.y, sp.w+sp.x+1, sp.h+sp.y+1, 0)
			g.loadMapObjs(g.loadDun("l2/blind2"), 2*sp.x, 2*sp.y)
		}
		if g.available(QBlood) {
			g.addBookLever(sp.x, sp.y+3, sp.x+2, sp.y+7, 0)
			g.addObject(ObjPEDISTAL, 2*sp.x+25, 2*sp.y+32)
		}
		g.initRndBarrels()
	case DTypeCaves:
		g.addLevelObjs(DTypeCaves)
		g.initRndBarrels()
	case DTypeHell:
		if g.available(QWarlord) {
			g.addBookLever(sp.x, sp.y, sp.x+sp.w, sp.y+sp.h, 0)
			g.loadMapObjs(g.loadDun("l4/warlord"), 2*sp.x, 2*sp.y)
		}
		if g.available(QBetrayer) {
			g.addLazStand()
		}
		g.initRndBarrels()
		g.addL4Goodies()
	}
	g.initRndLocObj(5, 10, ObjCHEST1)
	g.initRndLocObj(3, 6, ObjCHEST2)
	g.initRndLocObj(1, 5, ObjCHEST3)
	if g.ltype != DTypeHell {
		g.addObjTraps()
	}
	if g.ltype > DTypeCathedral {
		g.addChestTraps()
	}
	g.initObjFlag = false
}

// setMapObjects ports SetMapObjects: a set level's objects.
func (g *gen) setMapObjects(d dun, startx, starty int) {
	g.clrAllObjects()
	g.initObjFlag = true
	g.loadMapObjs(d, startx, starty)
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
	o := &Object{Kind: kind, X: ox, Y: oy, Index: oi, Solid: d.Solid, Missile: d.Missile, SelFlag: d.SelFlag, Active: true}
	g.objects[oi] = o
	if d.AnimFlag != 0 {
		g.rnd(d.AnimDelay) // _oAnimCnt
		o.AnimFrame = g.rnd(d.AnimLen-1) + 1
	} else {
		o.AnimFrame = d.AnimDelay
	}
	switch kind {
	case ObjL1LDOOR, ObjL1RDOOR:
		o.DoorFlag = true
		o.Var1 = g.pieceAt(ox, oy) + 1
		if kind == ObjL1LDOOR {
			o.Var2 = g.pieceAt(ox, oy-1) + 1
		} else {
			o.Var2 = g.pieceAt(ox-1, oy) + 1
		}
		o.Var4 = 0
	case ObjL2LDOOR, ObjL2RDOOR:
		o.DoorFlag = true
		if kind == ObjL2LDOOR {
			g.objSetMicro(ox, oy, 538)
		} else {
			g.objSetMicro(ox, oy, 540)
		}
		o.Var4 = 0
	case ObjL3LDOOR, ObjL3RDOOR:
		o.DoorFlag = true
		if kind == ObjL3LDOOR {
			g.objSetMicro(ox, oy, 531)
		} else {
			g.objSetMicro(ox, oy, 534)
		}
		o.Var4 = 0
	case ObjBOOK2R:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.setPiece.x, g.setPiece.y, g.setPiece.w+g.setPiece.x+1, g.setPiece.h+g.setPiece.y+1
		o.Var6 = o.AnimFrame + 1
	case ObjCHEST1, ObjCHEST2, ObjCHEST3, ObjTCHEST1, ObjTCHEST2, ObjTCHEST3:
		if g.rnd(2) == 0 {
			o.AnimFrame += 3
		}
		o.RndSeed = g.rng.Seed()
		n := map[int]int{ObjCHEST1: 1, ObjTCHEST1: 1, ObjCHEST2: 2, ObjTCHEST2: 2, ObjCHEST3: 3, ObjTCHEST3: 3}[kind]
		if g.setlevel {
			o.Var1 = n
		} else {
			o.Var1 = g.rnd(n + 1)
		}
		o.Var2 = g.rnd(8)
		o.Var4 = 0
	case ObjSARC:
		g.setDObject(ox, oy-1, -(oi + 1))
		o.ExtraCells = [][2]int{{ox, oy - 1}}
		o.Var1 = g.rnd(10)
		o.RndSeed = g.rng.Seed()
		if o.Var1 >= 8 {
			o.Var2 = g.preSpawnSkeleton()
		}
	case ObjFLAMEHOLE:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.trapid, 0, g.trapdir, 0
	case ObjFLAMELVR:
		o.Var1, o.Var2 = g.trapid, misFlameC
	case ObjWATER:
		o.AnimFrame = 1
	case ObjTRAPL, ObjTRAPR:
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
	case ObjBARREL, ObjBARRELEX:
		o.Var1 = 0
		o.RndSeed = g.rng.Seed()
		o.Var2 = g.rnd(10)
		o.Var3 = g.rnd(3)
		if o.Var2 >= 8 {
			o.Var4 = g.preSpawnSkeleton()
		}
	case ObjSHRINEL, ObjSHRINER:
		o.PreFlag = true
		for {
			v := g.rnd(len(ShrineNames))
			if g.currlevel <= shrineMax[v] && shrineAvail[v] != 2 {
				o.Var1 = v
				break
			}
		}
		if g.rnd(2) != 0 {
			o.AnimFrame = 12
		}
	case ObjBOOKCASEL, ObjBOOKCASER:
		o.RndSeed = g.rng.Seed()
		o.PreFlag = true
	case ObjSKELBOOK, ObjBOOKSTAND, ObjBLOODFTN, ObjGOATSHRINE, ObjCAULDRON, ObjTEARFTN:
		o.RndSeed = g.rng.Seed()
	case ObjDECAP:
		o.RndSeed = g.rng.Seed()
		o.AnimFrame = g.rnd(8) + 1
		o.PreFlag = true
	case ObjPURIFYINGFTN, ObjMURKYFTN:
		o.ExtraCells = [][2]int{{ox, oy - 1}, {ox - 1, oy}, {ox - 1, oy - 1}}
		for _, c := range o.ExtraCells {
			g.setDObject(c[0], c[1], -1-oi)
		}
		o.RndSeed = g.rng.Seed()
	case ObjARMORSTAND, ObjWARARMOR:
		if !g.armorFlag {
			o.AnimDisabled, o.SelFlag = true, 0
		}
		o.RndSeed = g.rng.Seed()
	case ObjBOOK2L:
		if g.setlevel && g.setLvlNum == SLVileBetrayer {
			o.AnimFrame = 4
		}
	case ObjMCIRCLE1, ObjMCIRCLE2:
		o.RndSeed = g.rng.Seed()
		o.PreFlag = true
		o.Var6, o.Var5 = 0, 1
	case ObjBCROSS, ObjTBCROSS:
		o.RndSeed = g.rng.Seed()
	case ObjPEDISTAL:
		o.Var1, o.Var2, o.Var3, o.Var4 = g.setPiece.x, g.setPiece.y, g.setPiece.x+g.setPiece.w, g.setPiece.y+g.setPiece.h
	case ObjWARWEAP, ObjWEAPONRACK:
		if !g.weaponFlag {
			o.AnimDisabled, o.SelFlag = true, 0
		}
		o.RndSeed = g.rng.Seed()
	case ObjTNUDEM2:
		o.RndSeed = g.rng.Seed()
		o.AnimFrame = g.rnd(4) + 1
		o.PreFlag = true
	}
	g.nobjects++
}

// addStoryBook ports AddObject(OBJ_STORYBOOK) with AddStoryBook, which reseeds the RNG from level 16's seed.
func (g *gen) addStoryBook(x, y int, seeds [NumLevels]int32) {
	g.addObject(ObjSTORYBOOK, x, y)
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

// objSetMicro ports ObjSetMicro's piece change (D1 piece number pn).
func (g *gen) objSetMicro(x, y, pn int) {
	if x, y, ok := flat(x, y); ok {
		g.piece[x][y] = pn - 1
	}
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
		for i := range offsetX {
			x, y := m.X+offsetX[i], m.Y+offsetY[i]
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
func (g *gen) objectAt(x, y int) *Object {
	if oi := g.dObj(x, y); oi > 0 && int(oi) <= maxObjects {
		return g.objects[oi-1]
	}
	return nil
}

// hangTorchesOnWalls moves the two wall torches AddL2Torches hangs off a wall onto the wall itself. D1 puts
// them on the floor cell in front, where its 3/4 view still draws them against the wall behind; top-down that
// leaves a torch standing in the open. Run after every object is placed, so nothing else shifts.
func (g *gen) hangTorchesOnWalls() {
	back := map[int][2]int{ObjTORCHL: {1, 0}, ObjTORCHR: {0, 1}} // the cell AddL2Torches stepped off
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

func objKindOrNone(o *Object) int {
	if o == nil {
		return -1
	}
	return o.Kind
}
