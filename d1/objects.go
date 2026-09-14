package d1

// Object kinds are D1's OBJ_ indexes (rows of xl_obj). Decorations (candles, banners, crosses, empty stands)
// are not placed (decision 12), but their rolls still happen so levels stay D1-shaped.
const (
	ObjChest1     = 5
	ObjChest2     = 6
	ObjChest3     = 7
	ObjSarc       = 48
	ObjBarrel     = 57
	ObjBarrelEx   = 58
	ObjShrineL    = 59
	ObjShrineR    = 60
	ObjSkelBook   = 61
	ObjBookcaseL  = 62
	ObjBookcaseR  = 63
	ObjBookstand  = 64
	ObjBloodFtn   = 66
	ObjDecap      = 67
	ObjPurifyFtn  = 76
	ObjArmorStand = 77
	ObjGoatShrine = 79
	ObjCauldron   = 80
	ObjMurkyFtn   = 81
	ObjTearFtn    = 82
	ObjWeaponRack = 92
)

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
)

var themeGood = [4]int{ThemeGoatShrine, ThemeShrine, ThemeSkelRoom, ThemeLibrary}

// Object is a placed D1 object (ObjectStruct minus graphics), in level cell coordinates.
type Object struct {
	Kind, X, Y       int
	Var1, Var2, Var3 int  // _oVar1-3: chest item count and kind, shrine type, barrel contents
	Skeleton         int  // pre-spawned skeleton monster type, -1 = none
	Active           bool // _oSelFlag: can still be used
	Index            int  // creation order on the level; identifies the object in saves
}

// World is what object and theme placement needs from a dungeon level (cell coordinates).
type World interface {
	Occupied(x, y int) bool // monster, player, object or item
	AddObject(o *Object)
	AddMonster(mt, x, y int)
	AddItem(it *Item, x, y int)
}

// ShrineNames are D1's shrinestrs.
var ShrineNames = [26]string{"Mysterious", "Hidden", "Gloomy", "Weird", "Magical", "Stone", "Religious", "Enchanted",
	"Thaumaturgic", "Fascinating", "Cryptic", "Magical", "Eldritch", "Eerie", "Divine", "Holy", "Sacred", "Spiritual",
	"Spooky", "Abandoned", "Creepy", "Quiet", "Secluded", "Ornate", "Glimmering", "Tainted"}

// ShrineMessages are the MsgStrings each shrine shows.
var ShrineMessages = [26]string{"Some are weakened as one grows strong", "New strength is forged through destruction",
	"Those who defend seldom attack", "The sword of justice is swift and sharp", "While the spirit is vigilant the body thrives",
	"The powers of mana refocused renews", "Time cannot diminish the power of steel", "Magic is not always what it seems to be",
	"What once was opened now is closed", "Intensity comes at the cost of wisdom", "Arcane power brings destruction",
	"While the spirit is vigilant the body thrives", "Crimson and Azure become as the sun", "Knowledge and wisdom at the cost of self",
	"Drink and be refreshed", "Wherever you go, there you are", "Energy comes at the cost of wisdom",
	"Riches abound when least expected", "Where avarice fails, patience gains reward", "The hands of men may be guided by fate",
	"Strength is bolstered by heavenly faith", "The essence of life flows from within", "The way is made clear when viewed from above",
	"Salvation comes at the cost of wisdom", "Mysteries are revealed in the light of reason", "Those who are last may yet be first"}

var (
	shrineMax   = [26]int{16, 16, 16, 16, 16, 16, 16, 8, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16, 16}
	shrineAvail = [26]int{0, 0, 1, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 2} // 2 = multiplayer only
)

// RandomShrine ports AddShrine's and FindValidShrine's roll (goat shrines and cauldrons never pick Thaumaturgic).
func (g *Generator) RandomShrine(dlvl int, goat bool) int {
	for {
		st := g.random(26)
		if dlvl <= shrineMax[st] && shrineAvail[st] != 2 && !(goat && st == 8) {
			return st
		}
	}
}

// Placer ports InitThemes/HoldThemeRooms/InitObjects/CreateThemeRooms for one level.
type Placer struct {
	G     *Generator
	W     World
	L     *Level
	Types []int // the level's monster types
	Dlvl  int

	lt       int // leveltype: 1 cathedral .. 4 hell
	objIndex int
	themes   []theme
	tx, ty   int // themex, themey (dPiece coordinates)
	tvar     int // themeVar1

	armor, bFountain, cauldron, mFountain, pFountain, tFountain, treasure, weapon bool
}

type theme struct{ ttype, ttval int }

func (g *Generator) NewPlacer(w World, lvl *Level, dlvl int, types []int) *Placer {
	return &Placer{G: g, W: w, L: lvl, Types: types, Dlvl: dlvl, lt: (dlvl-1)/4 + 1,
		armor: true, bFountain: true, cauldron: true, mFountain: true, pFountain: true, tFountain: true, treasure: true, weapon: true}
}

// The theme and object code works in D1's dPiece coordinates; cell = piece - 16.
func (p *Placer) cell(x, y int) (int, int, bool) {
	cx, cy := x-16, y-16
	return cx, cy, cx >= 0 && cy >= 0 && cx < LevelSize && cy < LevelSize
}

// solid: anything but open floor. Closed doors are solid in D1, which is what makes rooms enclosed.
func (p *Placer) solid(x, y int) bool {
	cx, cy, ok := p.cell(x, y)
	return !ok || p.L.Cells[cx][cy] != CellFloor
}

func (p *Placer) trans(x, y int) int {
	if cx, cy, ok := p.cell(x, y); ok {
		return p.L.Trans[cx][cy]
	}
	return 0
}

func (p *Placer) occupied(x, y int) bool {
	cx, cy, ok := p.cell(x, y)
	return !ok || p.W.Occupied(cx, cy)
}

func (p *Placer) held(x, y int) bool {
	cx, cy, ok := p.cell(x, y)
	return ok && p.L.Held[cx][cy]
}

// trapWall stands in for nTrapTable (wall pieces that can hold an object): a solid cell.
// ponytail: D1's per-piece table isn't in the source; any wall qualifies.
func (p *Placer) trapWall(x, y int) bool { return p.solid(x, y) }

func isSkel(m MonsterData) bool {
	return m.Family == "SkelAxe" || m.Family == "SkelBow" || m.Family == "SkelSd"
}
func isGoat(m MonsterData) bool { return m.Family == "GoatMace" || m.Family == "GoatBow" }

// skelType ports PreSpawnSkeleton's roll: a random skeleton type of the level, -1 if there is none.
func (p *Placer) skelType() int {
	var skels []int
	for _, mt := range p.Types {
		if isSkel(p.G.D.Monsters[mt]) {
			skels = append(skels, mt)
		}
	}
	if len(skels) == 0 {
		return -1
	}
	return skels[p.G.random(len(skels))]
}

func (p *Placer) addObject(kind, x, y int) *Object {
	cx, cy, ok := p.cell(x, y)
	if !ok || p.W.Occupied(cx, cy) {
		return nil
	}
	g := p.G
	o := &Object{Kind: kind, X: cx, Y: cy, Skeleton: -1, Active: true, Index: p.objIndex}
	p.objIndex++
	switch kind {
	case ObjChest1, ObjChest2, ObjChest3:
		g.random(2) // animation frame
		o.Var1 = g.random(kind - ObjChest1 + 2)
		o.Var2 = g.random(8)
	case ObjSarc:
		o.Var1 = g.random(10)
		if o.Var1 >= 8 {
			o.Skeleton = p.skelType()
		}
	case ObjBarrel, ObjBarrelEx:
		o.Var2 = g.random(10)
		o.Var3 = g.random(3)
		if o.Var2 >= 8 {
			o.Skeleton = p.skelType()
		}
	case ObjShrineL, ObjShrineR:
		o.Var1 = g.RandomShrine(p.Dlvl, false)
		g.random(2) // animation frame
	case ObjDecap:
		g.random(8) // animation frame
	}
	p.W.AddObject(o)
	return o
}

// checkThemeRoom ports CheckThemeRoom for the cathedral: an enclosed 9-100 cell region without stairs.
func (p *Placer) checkThemeRoom(tv int) bool {
	for _, s := range [][2]int{p.L.StairsUp, p.L.StairsDn} {
		if p.L.Trans[s[0]][s[1]] == tv {
			return false
		}
	}
	n := 0
	for x := 0; x < LevelSize; x++ {
		for y := 0; y < LevelSize; y++ {
			if p.L.Trans[x][y] == tv {
				if p.L.Held[x][y] {
					return false
				}
				n++
			}
		}
	}
	if p.lt == 1 && (n < 9 || n > 100) {
		return false
	}
	for x := 0; x < LevelSize; x++ {
		for y := 0; y < LevelSize; y++ {
			if p.L.Trans[x][y] != tv || p.solid(x+16, y+16) {
				continue
			}
			for _, d := range [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}} {
				nx, ny := x+d[0], y+d[1]
				if !p.solid(nx+16, ny+16) && p.L.Trans[nx][ny] != tv {
					return false
				}
			}
		}
	}
	return true
}

func (p *Placer) checkThemeReqs(t int) bool {
	switch t {
	case ThemeShrine, ThemeSkelRoom, ThemeLibrary:
		return p.lt != 3 && p.lt != 4
	case ThemeBloodFountain:
		return p.bFountain
	case ThemePurifyingFountain:
		return p.pFountain
	case ThemeArmorStand, ThemeWeaponRack:
		return p.lt != 1
	case ThemeCauldron:
		return p.lt == 4 && p.cauldron
	case ThemeMurkyFountain:
		return p.mFountain
	case ThemeTearFountain:
		return p.tFountain
	}
	return true
}

func (p *Placer) specialThemeFit(i, t int) bool {
	rv := p.checkThemeReqs(t)
	switch t {
	case ThemeShrine, ThemeLibrary:
		return rv && p.tfitShrine(i)
	case ThemeSkelRoom:
		return rv && p.tfitSkelRoom(i)
	case ThemeTreasure:
		rv = p.treasure
		p.treasure = false
	case ThemeTorture, ThemeDecapitated, ThemeArmorStand, ThemeBrnCross, ThemeWeaponRack:
		return rv && p.tfitObj3(i)
	case ThemeBloodFountain:
		if rv = rv && p.tfitObj5(i); rv {
			p.bFountain = false
		}
	case ThemePurifyingFountain:
		if rv = rv && p.tfitObj5(i); rv {
			p.pFountain = false
		}
	case ThemeGoatShrine:
		return rv && p.tfitGoatShrine(i)
	case ThemeCauldron:
		if rv = rv && p.tfitObj5(i); rv {
			p.cauldron = false
		}
	case ThemeMurkyFountain:
		if rv = rv && p.tfitObj5(i); rv {
			p.mFountain = false
		}
	case ThemeTearFountain:
		if rv = rv && p.tfitObj5(i); rv {
			p.tFountain = false
		}
	}
	return rv
}

// tfitShrine ports TFit_Shrine: a spot in front of a wall, above or to the left.
func (p *Placer) tfitShrine(i int) bool {
	tv := p.themes[i].ttval
	for y := 0; y < 112; y++ {
		for x := 0; x < 112; x++ {
			if p.trans(x, y) != tv {
				continue
			}
			if p.trapWall(x, y-1) && !p.solid(x-1, y) && !p.solid(x+1, y) && p.trans(x-1, y) == tv && p.trans(x+1, y) == tv &&
				!p.occupied(x-1, y-1) && !p.occupied(x+1, y-1) {
				p.tx, p.ty, p.tvar = x, y, 1
				return true
			}
			if p.trapWall(x-1, y) && !p.solid(x, y-1) && !p.solid(x, y+1) && p.trans(x, y-1) == tv && p.trans(x, y+1) == tv &&
				!p.occupied(x-1, y-1) && !p.occupied(x-1, y+1) {
				p.tx, p.ty, p.tvar = x, y, 2
				return true
			}
		}
	}
	return false
}

// tfitObj5 ports TFit_Obj5: the first cell with a free 5×5 area inside the room (D1's countdown never advances).
func (p *Placer) tfitObj5(i int) bool {
	tv := p.themes[i].ttval
	p.G.random(5)
	for y := 0; y < 112; y++ {
		for x := 0; x < 112; x++ {
			if p.trans(x, y) != tv || p.solid(x, y) {
				continue
			}
			ok := true
			for dy := -2; dy <= 2 && ok; dy++ {
				for dx := -2; dx <= 2 && ok; dx++ {
					ok = !p.solid(x+dx, y+dy) && p.trans(x+dx, y+dy) == tv
				}
			}
			if ok {
				p.tx, p.ty = x, y
				return true
			}
		}
	}
	return false
}

func (p *Placer) checkThemeObj3(x, y, i, f int) bool {
	tv := p.themes[i].ttval
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if x+dx < 0 || y+dy < 0 || p.solid(x+dx, y+dy) || p.trans(x+dx, y+dy) != tv || p.occupied(x+dx, y+dy) {
				return false
			}
			if f != -1 && p.G.random(f) == 0 {
				return false
			}
		}
	}
	return true
}

// tfitObj3 ports TFit_Obj3.
func (p *Placer) tfitObj3(i int) bool {
	f := [4]int{4, 4, 3, 5}[p.lt-1]
	for y := 1; y < 111; y++ {
		for x := 1; x < 111; x++ {
			if p.checkThemeObj3(x, y, i, f) {
				p.tx, p.ty = x, y
				return true
			}
		}
	}
	return false
}

func (p *Placer) firstType(ok func(MonsterData) bool) int {
	for _, mt := range p.Types {
		if ok(p.G.D.Monsters[mt]) {
			return mt
		}
	}
	return -1
}

func (p *Placer) tfitSkelRoom(i int) bool {
	if p.lt != 1 && p.lt != 2 {
		return false
	}
	if p.tvar = p.firstType(isSkel); p.tvar < 0 {
		return false
	}
	return p.tfitObj5(i)
}

func (p *Placer) tfitGoatShrine(i int) bool {
	if p.tvar = p.firstType(isGoat); p.tvar < 0 {
		return false
	}
	return p.tfitObj5(i)
}

// InitThemes picks theme rooms and their themes: enclosed rooms in the cathedral, the rooms the generator placed
// elsewhere. ponytail: Zhar's library on level 8 comes with his quest.
func (p *Placer) InitThemes() {
	if p.Dlvl == 16 {
		return
	}
	tvs := p.L.Themes
	if p.lt == 1 {
		tvs = nil
		for tv := 0; tv < 256 && len(tvs) < 50; tv++ {
			if p.checkThemeRoom(tv) {
				tvs = append(tvs, tv)
			}
		}
	}
	for _, tv := range tvs {
		i := len(p.themes)
		p.themes = append(p.themes, theme{ttval: tv})
		t := themeGood[p.G.random(4)]
		for !p.specialThemeFit(i, t) {
			t = p.G.random(17)
		}
		p.themes[i].ttype = t
	}
}

// HoldThemeRooms ports HoldThemeRooms: monsters and random objects stay out of theme rooms.
func (p *Placer) HoldThemeRooms() {
	for _, t := range p.themes {
		for x := 0; x < LevelSize; x++ {
			for y := 0; y < LevelSize; y++ {
				if p.L.Trans[x][y] == t.ttval {
					p.L.Held[x][y] = true
				}
			}
		}
	}
}

func (p *Placer) rndLocOk(x, y int) bool {
	return !p.occupied(x, y) && !p.held(x, y) && !p.solid(x, y)
}

func (p *Placer) areaOk(x, y, x1, y1, x2, y2 int) bool {
	for dy := y1; dy <= y2; dy++ {
		for dx := x1; dx <= x2; dx++ {
			if !p.rndLocOk(x+dx, y+dy) {
				return false
			}
		}
	}
	return true
}

// rndLoc ports InitRndLocObj (3×3 free) and InitRndLocBigObj (3×4 free).
func (p *Placer) rndLoc(min, max, kind int, big bool) {
	y1 := -1
	if big {
		y1 = -2
	}
	for n := p.G.random(max-min) + min; n > 0; n-- {
		for try := 0; try < 10000; try++ { // D1 loops until it fits
			x, y := p.G.random(80)+16, p.G.random(80)+16
			if p.areaOk(x, y, -1, y1, 1, 1) {
				p.addObject(kind, x, y)
				break
			}
		}
	}
}

// rndBarrels ports InitRndBarrels: 3-7 barrels, each starting a random chain of more.
func (p *Placer) rndBarrels() {
	bxadd := [8]int{-1, 0, 1, -1, 1, -1, 0, 1}
	byadd := [8]int{-1, -1, -1, 0, 0, 1, 1, 1}
	for n := p.G.random(5) + 3; n > 0; n-- {
		x, y := 0, 0
		for try := 0; try < 10000; try++ {
			if x, y = p.G.random(80)+16, p.G.random(80)+16; p.rndLocOk(x, y) {
				break
			}
		}
		kind := ObjBarrelEx
		if p.G.random(4) != 0 {
			kind = ObjBarrel
		}
		p.addObject(kind, x, y)
		for c, found := 1, true; p.G.random(c>>1) == 0 && found; {
			found = false
			for t := 0; !found && t < 3; t++ {
				d := p.G.random(8)
				x, y = x+bxadd[d], y+byadd[d]
				found = p.rndLocOk(x, y)
			}
			if found {
				kind = ObjBarrelEx
				if p.G.random(5) != 0 {
					kind = ObjBarrel
				}
				p.addObject(kind, x, y)
				c++
			}
		}
	}
}

// InitObjects ports InitObjects' random objects. ponytail: traps (AddObjTraps/AddChestTraps) are deferred with
// trap disarming; quest objects come with their quests; torches, hooked and tortured bodies are decoration.
func (p *Placer) InitObjects() {
	if p.Dlvl == 16 {
		return
	}
	if p.lt == 1 {
		p.rndLoc(10, 15, ObjSarc, true)
	}
	p.rndBarrels()
	if p.lt == 4 { // AddL4Goodies
		p.rndLoc(2, 6, ObjDecap, false)
		p.rndLoc(1, 3, ObjCauldron, false)
	}
	p.rndLoc(5, 10, ObjChest1, false)
	p.rndLoc(3, 6, ObjChest2, false)
	p.rndLoc(1, 5, ObjChest3, false)
}

// placeThemeMonsts ports PlaceThemeMonsts: one random monster type scattered over the room.
func (p *Placer) placeThemeMonsts(i, f int) {
	if len(p.Types) == 0 {
		return
	}
	mt := p.Types[p.G.random(len(p.Types))]
	tv := p.themes[i].ttval
	for y := 0; y < 112; y++ {
		for x := 0; x < 112; x++ {
			if p.trans(x, y) == tv && !p.solid(x, y) && !p.occupied(x, y) && p.G.random(f) == 0 {
				cx, cy, _ := p.cell(x, y)
				p.W.AddMonster(mt, cx, cy)
			}
		}
	}
}

// skeleton ports SpawnSkeleton for theme rooms: a skeleton at the spot, or on a free neighbour.
func (p *Placer) skeleton(x, y int) {
	mt := p.skelType()
	if mt < 0 {
		return
	}
	var spots [][2]int
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if !p.solid(x+dx, y+dy) && !p.occupied(x+dx, y+dy) {
				spots = append(spots, [2]int{x + dx, y + dy})
			}
		}
	}
	if !p.solid(x, y) && !p.occupied(x, y) {
		spots = [][2]int{{x, y}}
	}
	if len(spots) > 0 {
		s := spots[p.G.random(len(spots))]
		cx, cy, _ := p.cell(s[0], s[1])
		p.W.AddMonster(mt, cx, cy)
	}
}

// eachObj3 visits room cells with a free 3×3 area, rolling f like the D1 theme loops.
func (p *Placer) eachObj3(i, f int, place func(x, y int)) {
	for y := 1; y < 111; y++ {
		for x := 1; x < 111; x++ {
			if p.trans(x, y) == p.themes[i].ttval && !p.solid(x, y) && p.checkThemeObj3(x, y, i, -1) && p.G.random(f) == 0 {
				place(x, y)
			}
		}
	}
}

// CreateThemeRooms ports CreateThemeRooms and the Theme_* functions.
func (p *Placer) CreateThemeRooms() {
	if p.Dlvl == 16 {
		return
	}
	g := p.G
	lt := p.lt - 1
	hell := p.Dlvl > 12
	for i, t := range p.themes {
		p.tx, p.ty = 0, 0
		tv := t.ttval
		switch t.ttype {
		case ThemeBarrel:
			r := [4]int{2, 6, 4, 8}[lt]
			for y := 0; y < 112; y++ {
				for x := 0; x < 112; x++ {
					if p.trans(x, y) == tv && !p.solid(x, y) && g.random(r) == 0 {
						kind := ObjBarrel
						if g.random(r) != 0 {
							kind = ObjBarrelEx
						}
						p.addObject(kind, x, y)
					}
				}
			}
			p.placeThemeMonsts(i, [4]int{5, 7, 3, 9}[lt])
		case ThemeShrine:
			p.tfitShrine(i)
			kind := ObjShrineL
			if p.tvar == 1 {
				kind = ObjShrineR
			}
			p.addObject(kind, p.tx, p.ty)
			p.placeThemeMonsts(i, [4]int{6, 6, 3, 9}[lt])
		case ThemeMonstPit:
			r := g.random(100) + 1
			x, y := 0, 0
			for r > 0 {
				if p.trans(x, y) == tv && !p.solid(x, y) {
					r--
				}
				if r > 0 {
					if x++; x == 112 {
						x = 0
						if y++; y == 112 {
							y = 0
						}
					}
				}
			}
			if cx, cy, ok := p.cell(x, y); ok {
				p.W.AddItem(g.FloorItem(p.Dlvl, true, hell), cx, cy)
			}
			p.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
		case ThemeSkelRoom:
			p.tfitSkelRoom(i)
			x, y := p.tx, p.ty
			f := [4]int{6, 7, 3, 9}[lt]
			for _, s := range [][3]int{{-1, -1, 1}, {0, -1, 0}, {1, -1, 1}, {-1, 0, 1}, {1, 0, 1}, {-1, 1, 1}, {0, 1, 0}, {1, 1, 1}} {
				if s[2] == 0 || g.random(f) != 0 { // otherwise a banner stands there
					p.skeleton(x+s[0], y+s[1])
				}
			}
			if !p.occupied(x, y-3) {
				p.addObject(ObjSkelBook, x, y-2)
			}
			if !p.occupied(x, y+3) {
				p.addObject(ObjSkelBook, x, y+2)
			}
		case ThemeTreasure:
			r := [4]int{4, 9, 7, 10}[lt]
			for y := 0; y < 112; y++ {
				for x := 0; x < 112; x++ {
					if p.trans(x, y) != tv || p.solid(x, y) {
						continue
					}
					cx, cy, _ := p.cell(x, y)
					rv := g.random(r)
					var last *Item
					if 2*g.random(r) == 0 { // sic
						last = g.TypeItem(p.Dlvl, ITypeGold, "", false)
						p.W.AddItem(last, cx, cy)
					}
					if rv == 0 {
						last = g.FloorItem(p.Dlvl, false, hell)
						p.W.AddItem(last, cx, cy)
					}
					if last != nil && rv >= r-2 && p.lt != 1 {
						last.Value >>= 1
					}
				}
			}
			p.placeThemeMonsts(i, [4]int{6, 8, 3, 7}[lt])
		case ThemeLibrary:
			p.tfitShrine(i)
			kind := ObjBookcaseL
			if p.tvar == 1 {
				kind = ObjBookcaseR
			}
			p.addObject(kind, p.tx, p.ty)
			lib := [4]int{1, 2, 2, 5}[lt]
			for y := 1; y < 111; y++ {
				for x := 1; x < 111; x++ {
					if p.checkThemeObj3(x, y, i, -1) && g.random(lib) == 0 {
						o := p.addObject(ObjBookstand, x, y)
						if read := g.random(2*lib) != 0; o != nil && read {
							o.Active = false // D1 leaves some books already read
						}
					}
				}
			}
			p.placeThemeMonsts(i, [4]int{5, 7, 3, 9}[min(lt+1, 3)]) // D1 bug: indexes by leveltype, not leveltype-1
		case ThemeTorture:
			p.eachObj3(i, [4]int{6, 8, 3, 8}[lt], func(x, y int) {}) // tortured bodies are decoration
			p.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeBloodFountain, ThemePurifyingFountain, ThemeCauldron, ThemeMurkyFountain, ThemeTearFountain:
			p.tfitObj5(i)
			kind := map[int]int{ThemeBloodFountain: ObjBloodFtn, ThemePurifyingFountain: ObjPurifyFtn, ThemeCauldron: ObjCauldron,
				ThemeMurkyFountain: ObjMurkyFtn, ThemeTearFountain: ObjTearFtn}[t.ttype]
			p.addObject(kind, p.tx, p.ty)
			f := [4]int{6, 7, 3, 9}[lt]
			if t.ttype == ThemeBloodFountain {
				f = [4]int{6, 8, 3, 9}[lt]
			}
			p.placeThemeMonsts(i, f)
		case ThemeDecapitated:
			p.eachObj3(i, [4]int{6, 8, 3, 8}[lt], func(x, y int) { p.addObject(ObjDecap, x, y) })
			p.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeArmorStand:
			if p.armor {
				p.tfitObj3(i)
				p.addObject(ObjArmorStand, p.tx, p.ty)
			}
			p.eachObj3(i, [4]int{6, 8, 3, 8}[lt], func(x, y int) {}) // empty stands are decoration
			p.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
			p.armor = false
		case ThemeGoatShrine:
			p.tfitGoatShrine(i)
			p.addObject(ObjGoatShrine, p.tx, p.ty)
			for y := p.ty - 1; y <= p.ty+1; y++ {
				for x := p.tx - 1; x <= p.tx+1; x++ {
					if p.trans(x, y) == tv && !p.solid(x, y) && (x != p.tx || y != p.ty) && !p.occupied(x, y) {
						cx, cy, _ := p.cell(x, y)
						p.W.AddMonster(p.tvar, cx, cy)
					}
				}
			}
		case ThemeBrnCross:
			p.eachObj3(i, [4]int{5, 7, 3, 8}[lt], func(x, y int) {}) // burning crosses are decoration
			p.placeThemeMonsts(i, [4]int{6, 8, 3, 9}[lt])
		case ThemeWeaponRack:
			if p.weapon {
				p.tfitObj3(i)
				p.addObject(ObjWeaponRack, p.tx, p.ty)
			}
			p.eachObj3(i, [4]int{6, 8, 5, 8}[lt], func(x, y int) {}) // empty racks are decoration
			p.placeThemeMonsts(i, [4]int{6, 7, 3, 9}[lt])
			p.weapon = false
		}
	}
}
