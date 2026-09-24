package dungen

import "rpg/src/rules"

// themes.cpp: theme rooms.

// Theme types (THEME_).
const (
	ThemeBarrel = iota
	ThemeShrine
	ThemeMonstPit
	ThemeSkelRoom
	ThemeTreasure
	ThemeLibrary
	ThemeTorture
	ThemeBloodFountain
	ThemeDecapitated
	ThemePurifyingFountain
	ThemeArmorStand
	ThemeGoatShrine
	ThemeCauldron
	ThemeMurkyFountain
	ThemeTearFountain
	ThemeBrnCross
	ThemeWeaponRack
	themeNone = -1
)

type theme struct {
	ttval int
	ttype int
}

var themeGood = [4]int{ThemeGoatShrine, ThemeShrine, ThemeSkelRoom, ThemeLibrary}

func (g *gen) tfitShrine(i int) bool {
	tv := g.rooms[i].ttval
	for xp, yp := 0, 0; ; {
		found := 0
		if int(g.transAt(xp, yp)) == tv {
			if g.tileFlags(xp, yp-1).HoldsTrap && !g.solid(xp-1, yp) && !g.solid(xp+1, yp) && int(g.transAt(xp-1, yp)) == tv &&
				int(g.transAt(xp+1, yp)) == tv && g.dObj(xp-1, yp-1) == 0 && g.dObj(xp+1, yp-1) == 0 {
				found = 1
			}
			if found == 0 && g.tileFlags(xp-1, yp).HoldsTrap && !g.solid(xp, yp-1) && !g.solid(xp, yp+1) &&
				int(g.transAt(xp, yp-1)) == tv && int(g.transAt(xp, yp+1)) == tv && g.dObj(xp-1, yp-1) == 0 && g.dObj(xp-1, yp+1) == 0 {
				found = 2
			}
		}
		if found != 0 {
			g.themex, g.themey, g.themeVar1 = xp, yp, found
			return true
		}
		if xp++; xp == 112 {
			xp = 0
			if yp++; yp == 112 {
				return false
			}
		}
	}
}

func (g *gen) tfitObj5(t int) bool {
	tv := g.rooms[t].ttval
	xp, yp := 0, 0
	r := g.rnd(5) + 1
	rs := r
	for r > 0 {
		found := false
		if int(g.transAt(xp, yp)) == tv && !g.solid(xp, yp) {
			found = true
			for dy := -2; dy <= 2 && found; dy++ {
				for dx := -2; dx <= 2 && found; dx++ {
					if g.solid(xp+dx, yp+dy) || int(g.transAt(xp+dx, yp+dy)) != tv {
						found = false
					}
				}
			}
		}
		if !found {
			if xp++; xp == 112 {
				xp = 0
				if yp++; yp == 112 {
					if r == rs {
						return false
					}
					yp = 0
				}
			}
			continue
		}
		r--
	}
	g.themex, g.themey = xp, yp
	return true
}

func (g *gen) tfitSkelRoom(t int) bool {
	if g.ltype != rules.DTypeCathedral && g.ltype != rules.DTypeCatacombs {
		return false
	}
	for i, mt := range g.mtypes {
		if g.isSkel(mt.mtype) {
			g.themeVar1 = i
			return g.tfitObj5(t)
		}
	}
	return false
}

func (g *gen) tfitGoatShrine(t int) bool {
	for i, mt := range g.mtypes {
		if g.isGoat(mt.mtype) {
			g.themeVar1 = i
			return g.tfitObj5(t)
		}
	}
	return false
}

func (g *gen) checkThemeObj3(xp, yp, t, f int) bool {
	tv := g.rooms[t].ttval
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			x, y := xp+dx, yp+dy
			if x < 0 || y < 0 || g.solid(x, y) || int(g.transAt(x, y)) != tv || g.dObj(x, y) != 0 {
				return false
			}
			if f != -1 && g.rnd(f) == 0 {
				return false
			}
		}
	}
	return true
}

func (g *gen) tfitObj3(t int) bool {
	f := [4]int{4, 4, 3, 5}[g.ltype-1]
	for yp := 1; yp < 111; yp++ {
		for xp := 1; xp < 111; xp++ {
			if g.checkThemeObj3(xp, yp, t, f) {
				g.themex, g.themey = xp, yp
				return true
			}
		}
	}
	return false
}

func (g *gen) checkThemeReqs(t int) bool {
	switch t {
	case ThemeShrine, ThemeSkelRoom, ThemeLibrary:
		return g.ltype != rules.DTypeCaves && g.ltype != rules.DTypeHell
	case ThemeBloodFountain:
		return g.bFountain
	case ThemePurifyingFountain:
		return g.pFountain
	case ThemeArmorStand, ThemeWeaponRack:
		return g.ltype != rules.DTypeCathedral
	case ThemeCauldron:
		return g.ltype == rules.DTypeHell && g.cauldron
	case ThemeMurkyFountain:
		return g.mFountain
	case ThemeTearFountain:
		return g.tFountain
	}
	return true
}

func (g *gen) specialThemeFit(i, t int) bool {
	rv := g.checkThemeReqs(t)
	switch t {
	case ThemeShrine, ThemeLibrary:
		return rv && g.tfitShrine(i)
	case ThemeSkelRoom:
		return rv && g.tfitSkelRoom(i)
	case ThemeBloodFountain, ThemePurifyingFountain, ThemeMurkyFountain, ThemeTearFountain, ThemeCauldron:
		if rv = rv && g.tfitObj5(i); rv {
			switch t {
			case ThemeBloodFountain:
				g.bFountain = false
			case ThemePurifyingFountain:
				g.pFountain = false
			case ThemeMurkyFountain:
				g.mFountain = false
			case ThemeTearFountain:
				g.tFountain = false
			case ThemeCauldron:
				g.cauldron = false
			}
		}
		return rv
	case ThemeGoatShrine:
		return rv && g.tfitGoatShrine(i)
	case ThemeTorture, ThemeDecapitated, ThemeArmorStand, ThemeBrnCross, ThemeWeaponRack:
		return rv && g.tfitObj3(i)
	case ThemeTreasure:
		rv = g.treasr
		g.treasr = false
		return rv
	}
	return rv
}

func (g *gen) checkThemeRoom(tv int) bool {
	for _, t := range g.trigs {
		if int(g.transAt(t.X, t.Y)) == tv {
			return false
		}
	}
	tarea := 0
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if int(g.trans[i][j]) != tv {
				continue
			}
			if g.flags[i][j]&bflagPopulated != 0 {
				return false
			}
			tarea++
		}
	}
	if g.ltype == rules.DTypeCathedral && (tarea < 9 || tarea > 100) {
		return false
	}
	for j := 0; j < 112; j++ {
		for i := 0; i < 112; i++ {
			if int(g.trans[i][j]) != tv || g.solid(i, j) {
				continue
			}
			for _, d := range [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				if int(g.transAt(i+d[0], j+d[1])) != tv && !g.solid(i+d[0], j+d[1]) {
					return false
				}
			}
		}
	}
	return true
}

// initThemes ports InitThemes.
func (g *gen) initThemes() {
	g.zharlib = -1
	g.rooms = nil
	g.armorFlag, g.bFountain, g.cauldron, g.mFountain, g.pFountain, g.tFountain, g.treasr = true, true, true, true, true, true, true
	g.weaponFlag = true
	if g.diabloLevel() {
		return
	}
	pick := func(i int) {
		t := themeGood[g.rnd(4)]
		for !g.specialThemeFit(i, t) {
			t = g.rnd(17)
		}
		g.rooms[i].ttype = t
	}
	if g.ltype == rules.DTypeCathedral {
		for i := 0; i < 256 && len(g.rooms) < 50; i++ {
			if g.checkThemeRoom(i) {
				g.rooms = append(g.rooms, theme{ttval: i})
				pick(len(g.rooms) - 1)
			}
		}
		return
	}
	for i := range g.themes {
		g.rooms = append(g.rooms, theme{ttval: int(g.themes[i].ttval), ttype: themeNone})
	}
	if g.available(rules.QZhar) {
		for j := range g.rooms {
			if g.specialThemeFit(j, ThemeLibrary) {
				g.rooms[j].ttype = ThemeLibrary
				g.zharlib = j
				break
			}
		}
	}
	for i := range g.rooms {
		if g.rooms[i].ttype == themeNone {
			pick(i)
		}
	}
}

// holdThemes ports HoldThemeRooms.
func (g *gen) holdThemes() {
	if g.diabloLevel() {
		return
	}
	if g.ltype != rules.DTypeCathedral {
		g.holdThemeRooms()
		return
	}
	for _, t := range g.rooms {
		v := int(int8(t.ttval))
		for y := 0; y < 112; y++ {
			for x := 0; x < 112; x++ {
				if int(g.trans[x][y]) == v {
					g.flags[x][y] |= bflagPopulated
				}
			}
		}
	}
}

func (g *gen) placeThemeMonsts(t, f int) {
	var scatter []int
	for i, mt := range g.mtypes {
		if mt.placeFlags&placeScatter != 0 {
			scatter = append(scatter, i)
		}
	}
	mtype := scatter[g.rnd(len(scatter))]
	for yp := 0; yp < 112; yp++ {
		for xp := 0; xp < 112; xp++ {
			if int(g.trans[xp][yp]) == g.rooms[t].ttval && !g.solid(xp, yp) && g.dItem[xp][yp] == 0 && g.dObject[xp][yp] == 0 && g.rnd(f) == 0 {
				g.addMonster(xp, yp, g.rnd(8), mtype, true)
			}
		}
	}
}

func (g *gen) eachThemeCell(t int, from, to int, fn func(x, y int)) {
	for yp := from; yp < to; yp++ {
		for xp := from; xp < to; xp++ {
			if int(g.trans[xp][yp]) == g.rooms[t].ttval && !g.solid(xp, yp) {
				fn(xp, yp)
			}
		}
	}
}

// spawnSkeleton ports SpawnSkeleton: a pre-spawned skeleton stands up at or next to (x, y).
func (g *gen) spawnSkeleton(slot, x, y int) {
	if slot < 0 {
		return
	}
	activate := func(x, y int) {
		if fx, fy, ok := flat(x, y); ok {
			g.dMonster[fx][fy] = slot + 1
		}
		m := g.monsters[slot]
		m.X, m.Y, m.Hidden = x, y, false
	}
	if g.posOkMonst(x, y) {
		activate(x, y)
		return
	}
	var ok [3][3]bool
	avail := false
	for j := 0; j < 3; j++ {
		for k := 0; k < 3; k++ {
			ok[k][j] = g.posOkMonst(x-1+k, y-1+j)
			avail = avail || ok[k][j]
		}
	}
	if !avail {
		return
	}
	rs := g.rnd(15) + 1
	xx, yy := 0, 0
	for rs > 0 {
		if ok[xx][yy] {
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
	activate(x-1+xx, y-1+yy)
}

func (g *gen) posOkMonst(x, y int) bool {
	if g.solidLoc(x, y) || g.dMon(x, y) != 0 {
		return false
	}
	o := g.objAt(x, y)
	return o == nil || !o.Solid
}

// createThemeRooms ports CreateThemeRooms and the Theme_* functions.
func (g *gen) createThemeRooms() {
	if g.diabloLevel() {
		return
	}
	g.initObjFlag = true
	lt := g.ltype - 1
	for i := range g.rooms {
		g.themex, g.themey = 0, 0
		tv := g.rooms[i].ttval
		switch g.rooms[i].ttype {
		case ThemeBarrel:
			r := [4]int{2, 6, 4, 8}[lt]
			g.eachThemeCell(i, 0, 112, func(x, y int) {
				if g.rnd(r) == 0 {
					kind := rules.ObjBARRELEX
					if g.rnd(r) == 0 {
						kind = rules.ObjBARREL
					}
					g.addObject(kind, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{5, 7, 3, 9}[lt])
		case ThemeShrine:
			g.tfitShrine(i)
			if g.themeVar1 == 1 {
				g.addObject(rules.ObjCANDLE2, g.themex-1, g.themey)
				g.addObject(rules.ObjSHRINER, g.themex, g.themey)
				g.addObject(rules.ObjCANDLE2, g.themex+1, g.themey)
			} else {
				g.addObject(rules.ObjCANDLE2, g.themex, g.themey-1)
				g.addObject(rules.ObjSHRINEL, g.themex, g.themey)
				g.addObject(rules.ObjCANDLE2, g.themex, g.themey+1)
			}
			g.placeThemeMonsts(i, [4]int{6, 6, 3, 9}[lt])
		case ThemeMonstPit:
			r := g.rnd(100) + 1
			ixp, iyp := 0, 0
			for r > 0 {
				if int(g.trans[ixp][iyp]) == tv && !g.solid(ixp, iyp) {
					r--
				}
				if r <= 0 {
					continue
				}
				if ixp++; ixp == 112 {
					ixp = 0
					if iyp++; iyp == 112 {
						iyp = 0
					}
				}
			}
			g.createRndItem(ixp, iyp, true)
			g.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
		case ThemeSkelRoom:
			g.tfitSkelRoom(i)
			xp, yp := g.themex, g.themey
			f := [4]int{6, 7, 3, 9}[lt]
			g.addObject(rules.ObjSKFIRE, xp, yp)
			for _, s := range [...][3]int{{-1, -1, rules.ObjBANNERL}, {0, -1, -1}, {1, -1, rules.ObjBANNERR}, {-1, 0, rules.ObjBANNERM},
				{1, 0, rules.ObjBANNERM}, {-1, 1, rules.ObjBANNERR}, {0, 1, -1}, {1, 1, rules.ObjBANNERL}} {
				if s[2] == -1 || g.rnd(f) != 0 {
					g.spawnSkeleton(g.preSpawnSkeleton(), xp+s[0], yp+s[1])
				} else {
					g.addObject(s[2], xp+s[0], yp+s[1])
				}
			}
			if g.dObj(xp, yp-3) == 0 {
				g.addObject(rules.ObjSKELBOOK, xp, yp-2)
			}
			if g.dObj(xp, yp+3) == 0 {
				g.addObject(rules.ObjSKELBOOK, xp, yp+2)
			}
		case ThemeTreasure:
			r := [4]int{4, 9, 7, 10}[lt]
			g.rng.Seed()
			g.eachThemeCell(i, 0, 112, func(x, y int) {
				rv := g.rnd(r)
				if 2*g.rnd(r) == 0 {
					g.createTypeItem(x, y, false, rules.ITypeGold, "")
				}
				if rv == 0 {
					g.createRndItem(x, y, false)
				}
				if (rv == 0 || rv >= r-2) && g.numitems > 0 {
					if last := g.itemSlots[g.itemactive[g.numitems-1]]; rv >= r-2 && g.ltype != rules.DTypeCathedral {
						last.Item.Value >>= 1
					}
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 8, 3, 7}[lt])
		case ThemeLibrary:
			g.tfitShrine(i)
			if g.themeVar1 == 1 {
				g.addObject(rules.ObjBOOKCANDLE, g.themex-1, g.themey)
				g.addObject(rules.ObjBOOKCASER, g.themex, g.themey)
				g.addObject(rules.ObjBOOKCANDLE, g.themex+1, g.themey)
			} else {
				g.addObject(rules.ObjBOOKCANDLE, g.themex, g.themey-1)
				g.addObject(rules.ObjBOOKCASEL, g.themex, g.themey)
				g.addObject(rules.ObjBOOKCANDLE, g.themex, g.themey+1)
			}
			lib := [4]int{1, 2, 2, 5}[lt]
			for yp := 1; yp < 111; yp++ {
				for xp := 1; xp < 111; xp++ {
					if g.checkThemeObj3(xp, yp, i, -1) && g.dMonster[xp][yp] == 0 && g.libraryStand(lib) {
						g.addObject(rules.ObjBOOKSTAND, xp, yp)
						if g.rnd(2*lib) != 0 {
							if o := g.objAt(xp, yp); o != nil {
								o.SelFlag = 0
								o.AnimFrame += 2
							}
						}
					}
				}
			}
			if !g.available(rules.QZhar) || i != g.zharlib {
				g.placeThemeMonsts(i, [5]int{5, 7, 3, 9, 0}[lt+1]) // D1 bug: indexed by leveltype, not leveltype-1
			}
		case ThemeTorture:
			f := [4]int{6, 8, 3, 8}[lt]
			g.eachThemeCell(i, 1, 111, func(x, y int) {
				if g.checkThemeObj3(x, y, i, -1) && g.rnd(f) == 0 {
					g.addObject(rules.ObjTNUDEM2, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeBloodFountain:
			g.tfitObj5(i)
			g.addObject(rules.ObjBLOODFTN, g.themex, g.themey)
			g.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeDecapitated:
			f := [4]int{6, 8, 3, 8}[lt]
			g.eachThemeCell(i, 1, 111, func(x, y int) {
				if g.checkThemeObj3(x, y, i, -1) && g.rnd(f) == 0 {
					g.addObject(rules.ObjDECAP, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemePurifyingFountain, ThemeCauldron, ThemeMurkyFountain, ThemeTearFountain:
			g.tfitObj5(i)
			g.addObject(map[int]int{ThemePurifyingFountain: rules.ObjPURIFYINGFTN, ThemeCauldron: rules.ObjCAULDRON,
				ThemeMurkyFountain: rules.ObjMURKYFTN, ThemeTearFountain: rules.ObjTEARFTN}[g.rooms[i].ttype], g.themex, g.themey)
			g.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
		case ThemeArmorStand:
			if g.armorFlag {
				g.tfitObj3(i)
				g.addObject(rules.ObjARMORSTAND, g.themex, g.themey)
			}
			f := [4]int{6, 8, 3, 8}[lt]
			g.eachThemeCell(i, 0, 112, func(x, y int) {
				if g.checkThemeObj3(x, y, i, -1) && g.rnd(f) == 0 {
					g.addObject(rules.ObjARMORSTANDN, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
			g.armorFlag = false
		case ThemeGoatShrine:
			g.tfitGoatShrine(i)
			g.addObject(rules.ObjGOATSHRINE, g.themex, g.themey)
			for yy := g.themey - 1; yy <= g.themey+1; yy++ {
				for xx := g.themex - 1; xx <= g.themex+1; xx++ {
					if int(g.transAt(xx, yy)) == tv && !g.solid(xx, yy) && (xx != g.themex || yy != g.themey) {
						g.addMonster(xx, yy, 1, g.themeVar1, true) // DIR_SW
					}
				}
			}
		case ThemeBrnCross:
			f := [4]int{5, 7, 3, 8}[lt]
			g.eachThemeCell(i, 0, 112, func(x, y int) {
				if g.checkThemeObj3(x, y, i, -1) && g.rnd(f) == 0 {
					g.addObject(rules.ObjTBCROSS, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeWeaponRack:
			if g.weaponFlag {
				g.tfitObj3(i)
				g.addObject(rules.ObjWEAPONRACK, g.themex, g.themey)
			}
			f := [4]int{6, 8, 5, 8}[lt]
			g.eachThemeCell(i, 0, 112, func(x, y int) {
				if g.checkThemeObj3(x, y, i, -1) && g.rnd(f) == 0 {
					g.addObject(rules.ObjWEAPONRACKN, x, y)
				}
			})
			g.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
			g.weaponFlag = false
		}
	}
	g.initObjFlag = false
	if g.ltype == rules.DTypeHell && len(g.themes) > 0 {
		g.updateL4Trans()
	}
}

// libraryStand rolls whether a free library cell gets a bookstand: D1's 1 in lib, or the level's library_stands percent.
func (g *gen) libraryStand(lib int) bool {
	if g.libraryStands > 0 {
		return g.rnd(100) < g.libraryStands
	}
	return g.rnd(lib) == 0
}
