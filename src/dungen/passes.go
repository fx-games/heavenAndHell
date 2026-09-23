package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"
	"slices"

	"rpg/src/dungeon"
	"rpg/src/recfile"
)

// Passes are what any layout may run over the level it carved, each switched on by its own levels.rec knob.
// They work on the layout's grid; with scale 2 that grid is drawn two by two cells (D1's megatiles, toCells),
// which is how D1's level types are laid out at their own scale. Every level they touch ends connected.
type Passes struct {
	Grow         int          // grow rectangular rooms up to this side until single walls part them; 0: off
	HallChance   float64      // a grown room at least 6 wide and 8 long becomes a pillared hall (two arcades)
	Spine        bool         // D1's first cathedral room: up to three pillared chambers on a long pillared hall
	Mirror       bool         // mirror the layout left-right and top-bottom (D1 L4 builds a quadrant and mirrors it)
	ArchChance   float64      // a straight wall between two rooms becomes an arcade (`#.#.#`)
	LoopChance   float64      // chance each wall between two spots farther than LoopDistance apart on foot opens
	LoopDistance int          // on foot, in grid cells
	Lava         *NoiseConfig // its lakes and rivers become lava, ringed with floor; fords keep the level whole
	Posts        float64      // chance a room cell with only room or wall round it gets a pillar
	DoorChance   float64      // chance each doorway (a 1-wide gap, or a door the layout made) is a door; <0: leave doors be
	Canvas       int          // the layout is centred on a canvas this many cells square; 0: its own size
	Scale        int          // 2 draws each cell as 2x2 (D1's megatiles); 1 leaves it
	Themes       *ThemeRooms  // D1's theme rooms, walled in inside open ground once the level is laid out
	Pieces       []PieceSpec  // the level's set pieces
	Halls        [3]int       // open_halls: how many open rectangles of min..max cells a side to clear over the level
}

// staged is a layout with passes and a population: what a levels.rec record naming any of their knobs builds.
type staged struct {
	Layout
	Passes
	population *Population
	avail      func(quest string) bool // which quests' set pieces go in (WithQuests)
}

// PopulationOf is the population a level's configuration names, if any.
func PopulationOf(cfg dungeon.Config) (Population, bool) {
	if s, ok := cfg.(staged); ok && s.population != nil {
		return *s.population, true
	}
	return Population{}, false
}

// passKnobs are the fields that switch passes on: a record naming none of them builds its layout bare.
var passKnobs = []string{"grow", "hall_chance", "spine", "mirror", "arch_chance", "loop_chance", "loop_distance",
	"lava_rivers", "lava_water", "posts", "door_chance", "canvas", "scale", "theme_rooms", "set_piece", "open_halls"}

// withPasses parses a layout record, wrapping it in its passes when it names any.
type withPasses struct{ dungeon.Parser }

// WithPasses lets a layout generator's records turn the passes on.
func WithPasses(p dungeon.Parser) dungeon.Parser { return withPasses{p} }

func (w withPasses) Parse(r recfile.Record) (dungeon.Config, error) {
	cfg, err := w.Parser.Parse(r)
	if err != nil {
		return nil, err
	}
	pop, populated, err := ParsePopulation(r)
	if err != nil {
		return nil, err
	}
	if !populated && !slices.ContainsFunc(passKnobs, func(k string) bool { _, ok := dungeon.Field(r, k); return ok }) {
		return cfg, nil
	}
	l, ok := cfg.(Layout)
	if !ok {
		return nil, fmt.Errorf("levels.rec: passes need a layout generator")
	}
	p := Passes{LoopDistance: 8, DoorChance: -1, Scale: 1}
	var lavaRivers int
	lava := NoiseConfig{Scale: 6, Rock: 0.99, Water: 0.26, RiverWidth: 1, RiverMeander: 0.35}
	if err := parseKnobs(r, []intKnob{{"grow", &p.Grow}, {"loop_distance", &p.LoopDistance}, {"canvas", &p.Canvas},
		{"scale", &p.Scale}, {"lava_rivers", &lavaRivers}},
		[]floatKnob{{"hall_chance", &p.HallChance}, {"arch_chance", &p.ArchChance}, {"loop_chance", &p.LoopChance},
			{"posts", &p.Posts}, {"door_chance", &p.DoorChance}, {"lava_water", &lava.Water}}); err != nil {
		return nil, err
	}
	p.Spine = boolField(r, "spine")
	p.Mirror = boolField(r, "mirror")
	if _, ok := dungeon.Field(r, "lava_water"); ok || lavaRivers > 0 {
		lava.Rivers = lavaRivers
		p.Lava = &lava
	}
	if v, ok := dungeon.Field(r, "theme_rooms"); ok {
		t := ThemeRooms{RandomSize: boolField(r, "theme_room_random_size"), Doors: boolField(r, "theme_room_doors")}
		if _, err := fmt.Sscan(v, &t.Min, &t.Max, &t.Freq); err != nil || t.Min < 3 || t.Max < t.Min || t.Freq < 0 {
			return nil, fmt.Errorf("levels.rec: theme_rooms %q: want min max freq, 3 <= min <= max, freq >= 0", v)
		}
		p.Themes = &t
	}
	if v, ok := dungeon.Field(r, "open_halls"); ok {
		if _, err := fmt.Sscan(v, &p.Halls[0], &p.Halls[1], &p.Halls[2]); err != nil || p.Halls[1] < 1 || p.Halls[2] < p.Halls[1] {
			return nil, fmt.Errorf("levels.rec: open_halls %q: want count min max, 1 <= min <= max", v)
		}
	}
	if p.Pieces, err = parsePieces(r); err != nil {
		return nil, err
	}
	if p.Scale != 1 && p.Scale != 2 {
		return nil, fmt.Errorf("levels.rec: scale must be 1 or 2")
	}
	st := staged{Layout: l, Passes: p}
	if populated {
		st.population = &pop
	}
	return st, nil
}

func boolField(r recfile.Record, k string) bool { v, _ := dungeon.Field(r, k); return v == "true" }

// Build carves the layout and runs the passes the record switched on, in the order Passes lists them.
func (s staged) Build(rnd *rand.Rand, fsys fs.FS) (*DungeonMap, error) {
	dm, err := s.Layout.Build(rnd, fsys)
	if err != nil {
		return nil, err
	}
	p := s.Passes
	if p.bare() { // a population alone: the layout as it came
		dm.scale = p.Scale
		return dm, nil
	}
	m := fromDungeon(dm)
	if p.Grow > 0 {
		halls(m, growRooms(m, dm, p.Grow, rnd), p.HallChance, rnd)
	}
	var chambers [][2]int
	if p.Spine {
		chambers = spine(m, rnd)
	}
	if p.Mirror {
		m = mirror(m, rnd)
	}
	arches(m, p.ArchChance, rnd)
	loops(m, p.LoopChance, p.LoopDistance, rnd)
	openHalls(m, p.Halls, rnd)
	if p.Lava != nil {
		p.Lava.Width, p.Lava.Height = m.w, m.h
		if err := addLava(m, p.Lava, rnd); err != nil {
			return nil, err
		}
	}
	if p.Canvas > 0 {
		full := newMega(p.Canvas, p.Canvas)
		ox, oy := (full.w-m.w)/2, (full.h-m.h)/2
		if ox < 0 || oy < 0 {
			return nil, fmt.Errorf("dungen: layout %dx%d is larger than canvas %d", m.w, m.h, p.Canvas)
		}
		for y := 0; y < m.h; y++ {
			for x := 0; x < m.w; x++ {
				full.set(x+ox, y+oy, m.at(x, y))
			}
		}
		m = full
		for i := range chambers {
			chambers[i][0] += ox
			chambers[i][1] += oy
		}
	}
	pieces, err := s.placePieces(m, fsys, chambers, rnd)
	if err != nil {
		return nil, err
	}
	var themes [][4]int
	if p.Themes != nil {
		themes = themeRooms(m, *p.Themes, rnd)
	}
	addPosts(m, p.Posts, rnd) // after the theme rooms, as D1's substitutions come after them
	connectMega(m)
	if p.DoorChance >= 0 {
		addDoors(m, p.DoorChance, rnd)
	}
	out := m.dungeonMap()
	if p.Scale == 2 {
		out = toCells(m)
	}
	out.themeRooms, out.scale = themes, p.Scale
	return out, stampPieces(out, pieces, fsys, p.Scale)
}

// Megatile kinds.
const (
	mWall = iota
	mRoom
	mCorr
	mDoor
	mLava
	mPost   // floor with a pillar in its top-left cell: in a row, an arcade
	mPiece  // a set piece's footprint: open ground the passes leave alone
	mLocked // never carved, never joined (connectCells: a set piece's own cells)
)

// kindTiles is what each kind is on a DungeonMap drawn at its own scale.
var kindTiles = [...]DungeonTile{mWall: Wall, mRoom: Room, mCorr: Corridor, mDoor: Door, mLava: Lava, mPost: Pillar, mPiece: Room, mLocked: Wall}

func (m *mega) dungeonMap() *DungeonMap {
	dm := NewDungeonMap(m.w, m.h)
	for i, k := range m.t {
		dm.tiles[i] = kindTiles[k]
	}
	return dm
}

func fromDungeon(dm *DungeonMap) *mega {
	w, h := dm.GetSize()
	m := newMega(w, h)
	for i, t := range dm.tiles {
		switch t {
		case Room:
			m.t[i] = mRoom
		case Corridor, Shallows:
			m.t[i] = mCorr
		case Door:
			m.t[i] = mDoor
		case Lake, Lava: // ponytail: water becomes lava once passes run; keep an mLake when a watery level wants passes
			m.t[i] = mLava
		case Pillar:
			m.t[i] = mPost
		}
	}
	return m
}

// toCells draws megatiles as D1 draws them, two by two cells: a wall megatile keeps its wall on its top and left
// cells and gives its bottom and right cells to the floor beside it, so walls are one cell thick and a corridor
// one megatile wide is three cells wide. A door is one door cell in that wall line; it counts as wall for the
// cells around it, so nothing slips past it. A post is a floor megatile with a pillar in its top-left cell.
func toCells(m *mega) *DungeonMap {
	g := NewDungeonMap(2*m.w, 2*m.h)
	floorLike := func(k int) bool { return k != mWall && k != mDoor }
	for y := 0; y < g.height; y++ {
		for x := 0; x < g.width; x++ {
			i, j, a, b := x/2, y/2, x%2, y%2
			switch k := m.at(i, j); {
			case k == mPost && a == 0 && b == 0:
				g.tiles[x+y*g.width] = Pillar
			case mopen(k):
				g.SetRoom(x, y)
			case k == mLava:
				g.tiles[x+y*g.width] = Lava
			default: // wall or door: the cell is floor where it borders open ground on its right or bottom
				right, below, diag := floorLike(m.at(i+1, j)), floorLike(m.at(i, j+1)), floorLike(m.at(i+1, j+1))
				if a == 1 && right || b == 1 && below || a == 1 && b == 1 && diag {
					g.tiles[x+y*g.width] = lavaOr(m, i, j, a, b)
				} else if k == mDoor && a == 0 && b == 0 {
					g.SetDoor(x, y)
				}
			}
		}
	}
	return g
}

// lavaOr is what a wall megatile's given-away cell shows: the ground beside it that it was given to (right,
// else below, else diagonally), lava when that is lava.
func lavaOr(m *mega, i, j, a, b int) DungeonTile {
	next := m.at(i+1, j+1)
	switch floorLike := func(k int) bool { return k != mWall && k != mDoor }; {
	case a == 1 && floorLike(m.at(i+1, j)):
		next = m.at(i+1, j)
	case b == 1 && floorLike(m.at(i, j+1)):
		next = m.at(i, j+1)
	}
	if next == mLava {
		return Lava
	}
	return Room
}

type mega struct {
	w, h int
	t    []int
}

func newMega(w, h int) *mega { return &mega{w: w, h: h, t: make([]int, w*h)} }

func (m *mega) at(x, y int) int {
	if x < 0 || y < 0 || x >= m.w || y >= m.h {
		return mWall
	}
	return m.t[x+y*m.w]
}

func (m *mega) set(x, y, k int) {
	if x >= 0 && y >= 0 && x < m.w && y < m.h {
		m.t[x+y*m.w] = k
	}
}

// open is ground one walks on without a door; walkable adds doors.
func mopen(k int) bool     { return k == mRoom || k == mCorr || k == mPost || k == mPiece }
func mwalkable(k int) bool { return mopen(k) || k == mDoor }

type mrect struct{ x0, y0, x1, y1, cap int } // inclusive floor bounds, and the side it may grow to

// growRooms widens each room (to a side between maxSide/2 and maxSide, rolled per room) one row or column at a
// time, in turns, while the strip it takes holds no other room's floor and the strip beyond (its new wall) holds no room floor and at most two corridor cells: rooms end
// up one wall apart, and the corridors that crossed the gap between them become openings in that wall.
func growRooms(m *mega, dm *DungeonMap, maxSide int, rnd *rand.Rand) []mrect {
	var rs []mrect
	for _, r := range dm.AllRooms() {
		b := mrect{m.w, m.h, -1, -1, maxSide/2 + rnd.Intn(maxSide-maxSide/2+1)}
		for _, p := range r.GetAbsoluteFloorTiles() {
			b.x0, b.y0, b.x1, b.y1 = min(b.x0, p.X), min(b.y0, p.Y), max(b.x1, p.X), max(b.y1, p.Y)
		}
		rs = append(rs, b)
	}
	// strip counts room and corridor cells on the line at offset d beyond side s of r, one longer at each end.
	strip := func(r mrect, s, d int) (rooms, corrs int, inside bool) {
		inside = true
		cellAt := func(x, y int) {
			if x < 1 || y < 1 || x >= m.w-1 || y >= m.h-1 {
				inside = false
				return
			}
			switch m.at(x, y) {
			case mRoom, mDoor:
				rooms++
			case mCorr:
				corrs++
			}
		}
		switch s {
		case 0: // north
			for x := r.x0 - 1; x <= r.x1+1; x++ {
				cellAt(x, r.y0-d)
			}
		case 1: // south
			for x := r.x0 - 1; x <= r.x1+1; x++ {
				cellAt(x, r.y1+d)
			}
		case 2: // west
			for y := r.y0 - 1; y <= r.y1+1; y++ {
				cellAt(r.x0-d, y)
			}
		case 3: // east
			for y := r.y0 - 1; y <= r.y1+1; y++ {
				cellAt(r.x1+d, y)
			}
		}
		return
	}
	for grew := true; grew; {
		grew = false
		for i := range rs {
			for s := 0; s < 4; s++ {
				r := rs[i]
				if (s < 2 && r.y1-r.y0+2 > r.cap) || (s >= 2 && r.x1-r.x0+2 > r.cap) {
					continue
				}
				// the strip taken may hold only this room's own corners' neighbours: check the side's span
				roomsIn, _, ok1 := strip(r, s, 1)
				roomsBeyond, corrsBeyond, ok2 := strip(r, s, 2)
				if !ok1 || !ok2 || roomsIn > 0 || roomsBeyond > 0 || corrsBeyond > 2 {
					continue
				}
				switch s {
				case 0:
					r.y0--
				case 1:
					r.y1++
				case 2:
					r.x0--
				case 3:
					r.x1++
				}
				for y := r.y0; y <= r.y1; y++ {
					for x := r.x0; x <= r.x1; x++ {
						m.set(x, y, mRoom)
					}
				}
				rs[i], grew = r, true
			}
		}
	}
	return rs
}

// halls turns, with chance p, a grown room at least 6 wide and 8 long into a pillared hall: an arcade one
// megatile in from each long wall, as D1's l1GenerateHall draws its arch walls.
func halls(m *mega, rs []mrect, p float64, rnd *rand.Rand) {
	for _, r := range rs {
		w, h := r.x1-r.x0+1, r.y1-r.y0+1
		if min(w, h) < 6 || max(w, h) < 8 || rnd.Float64() >= p {
			continue
		}
		for y := r.y0; y <= r.y1; y++ {
			for x := r.x0; x <= r.x1; x++ {
				if w <= h && (x == r.x0+1 || x == r.x1-1) || w > h && (y == r.y0+1 || y == r.y1-1) {
					m.set(x, y, mPost)
				}
			}
		}
	}
}

// spine stamps D1's first cathedral room (l1FirstRoom, l1FillChambers, l1GenerateChamber) at D1's coordinates:
// a hall six megatiles wide across the level, arcades on its second and fifth rows, and up to three 10x10
// chambers along it, each with four pillars and arcades where the hall runs into it. Rooms it lands on lose that
// part; connect joins what is left of them.
func spine(m *mega, rnd *rand.Rand) (chambers [][2]int) {
	vertical := rnd.Intn(2) == 0
	ch := [3]bool{rnd.Intn(2) != 0, rnd.Intn(2) != 0, rnd.Intn(2) != 0}
	if !ch[0] || !ch[2] {
		ch[1] = true
	}
	set := func(x, y, k int) {
		if vertical {
			x, y = y, x
		}
		m.set(x, y, k)
	}
	x0, x1 := 1, 38
	if !ch[0] {
		x0 += 17
	}
	if !ch[2] {
		x1 -= 16
	}
	for x := x0; x <= x1; x++ {
		for y := 16; y <= 23; y++ {
			k := mRoom
			switch y {
			case 16, 23:
				k = mWall
			case 18, 21:
				k = mPost
			}
			set(x, y, k)
		}
	}
	for i, cx := range [3]int{0, 14, 28} {
		if !ch[i] {
			continue
		}
		const cy = 14
		for y := cy; y <= cy+11; y++ {
			for x := cx; x <= cx+11; x++ {
				k := mRoom
				if x == cx || x == cx+11 || y == cy || y == cy+11 {
					k = mWall
					if hall := x == cx && x0 < cx || x == cx+11 && x1 > cx+11; hall && y >= cy+2 && y <= cy+9 {
						k = mPost // the arcade the hall runs in through, open across its middle aisle
						if y == cy+5 || y == cy+6 {
							k = mRoom
						}
					}
				}
				set(x, y, k)
			}
		}
		for _, p := range [4][2]int{{4, 4}, {7, 4}, {4, 7}, {7, 7}} {
			set(cx+p[0], cy+p[1], mPost)
		}
		if vertical {
			chambers = append(chambers, [2]int{cy, cx})
		} else {
			chambers = append(chambers, [2]int{cx, cy})
		}
	}
	return chambers
}

// mirror doubles a quadrant into a 4-way symmetric level and opens the seams: the quadrant's edge walls meet
// back to back, so a few floor cells next to each seam are carried through it (connect makes sure).
func mirror(q *mega, rnd *rand.Rand) *mega {
	m := newMega(2*q.w, 2*q.h)
	for y := 0; y < q.h; y++ {
		for x := 0; x < q.w; x++ {
			k := q.at(x, y)
			m.set(x, y, k)
			m.set(m.w-1-x, y, k)
			m.set(x, m.h-1-y, k)
			m.set(m.w-1-x, m.h-1-y, k)
		}
	}
	var rows, cols []int // where floor meets the right and bottom edges of the quadrant
	for y := 0; y < q.h; y++ {
		if mopen(q.at(q.w-2, y)) {
			rows = append(rows, y)
		}
	}
	for x := 0; x < q.w; x++ {
		if mopen(q.at(x, q.h-2)) {
			cols = append(cols, x)
		}
	}
	for _, lines := range [][]int{rows, cols} {
		rnd.Shuffle(len(lines), func(i, j int) { lines[i], lines[j] = lines[j], lines[i] })
	}
	for i := 0; i < min(2, len(rows)); i++ {
		for _, y := range []int{rows[i], m.h - 1 - rows[i]} {
			m.set(q.w-1, y, mRoom)
			m.set(q.w, y, mRoom)
		}
	}
	for i := 0; i < min(2, len(cols)); i++ {
		for _, x := range []int{cols[i], m.w - 1 - cols[i]} {
			m.set(x, q.h-1, mRoom)
			m.set(x, q.h, mRoom)
		}
	}
	return m
}

// arches turns, with chance p, each straight run (3 or more) of wall with room floor on both sides into an
// arcade, D1's arch walls between cathedral rooms.
func arches(m *mega, p float64, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for _, vertical := range []bool{false, true} {
		at := func(a, b int) int { // a runs along the wall, b across it
			if vertical {
				return m.at(b, a)
			}
			return m.at(a, b)
		}
		along, across := m.w, m.h
		if vertical {
			along, across = m.h, m.w
		}
		for b := 1; b < across-1; b++ {
			for a := 0; a < along; {
				start := a
				for a < along && at(a, b) == mWall && at(a, b-1) == mRoom && at(a, b+1) == mRoom {
					a++
				}
				if a-start >= 3 && rnd.Float64() < p {
					for i := start; i < a; i++ {
						if vertical {
							m.set(b, i, mPost)
						} else {
							m.set(i, b, mPost)
						}
					}
				}
				if a == start {
					a++
				}
			}
		}
	}
}

// loops opens, with chance p, each wall megatile between floor on two opposite sides (walls on the other two)
// whose sides are more than dist megatiles apart on foot, or not joined at all: accretion's loop_distance and
// loop_chance, so the way back is not always the way in. Openings become doorways; addDoors may shut them.
func loops(m *mega, p float64, dist int, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for _, i := range rnd.Perm(len(m.t)) {
		x, y := i%m.w, i/m.w
		if m.t[i] != mWall {
			continue
		}
		var a, b [2]int
		switch {
		case mwalkable(m.at(x-1, y)) && mwalkable(m.at(x+1, y)) && m.at(x, y-1) == mWall && m.at(x, y+1) == mWall:
			a, b = [2]int{x - 1, y}, [2]int{x + 1, y}
		case mwalkable(m.at(x, y-1)) && mwalkable(m.at(x, y+1)) && m.at(x-1, y) == mWall && m.at(x+1, y) == mWall:
			a, b = [2]int{x, y - 1}, [2]int{x, y + 1}
		default:
			continue
		}
		if rnd.Float64() >= p {
			continue
		}
		if d := walk(m, a, b); d < 0 || d > dist {
			m.t[i] = mCorr
		}
	}
}

// walk is how many megatiles apart a and b are on foot, -1 if they are not joined.
func walk(m *mega, a, b [2]int) int {
	dist := map[[2]int]int{a: 0}
	for q := [][2]int{a}; len(q) > 0; q = q[1:] {
		c := q[0]
		if c == b {
			return dist[c]
		}
		for _, d := range mdirs {
			n := [2]int{c[0] + d[0], c[1] + d[1]}
			if _, seen := dist[n]; !seen && mwalkable(m.at(n[0], n[1])) {
				dist[n] = dist[c] + 1
				q = append(q, n)
			}
		}
	}
	return -1
}

var mdirs = [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}

// addLava turns the lakes and rivers of a noise map into lava, rock and floor alike, each lake that reaches the
// cave ringed with floor so the cave runs round it. Where lava cuts the level apart, connect lays a ford.
func addLava(m *mega, cfg *NoiseConfig, rnd *rand.Rand) error {
	nm, err := cfg.Build(rnd, nil)
	if err != nil {
		return err
	}
	lake := func(x, y int) bool {
		return x >= 1 && y >= 1 && x < m.w-1 && y < m.h-1 && nm.GetTile(x, y) == Lake
	}
	seen := make([]bool, len(m.t))
	for i := range m.t {
		if seen[i] || !lake(i%m.w, i/m.w) {
			continue
		}
		var pool []int
		seen[i] = true
		for q := []int{i}; len(q) > 0; q = q[1:] {
			pool = append(pool, q[0])
			for _, d := range mdirs {
				nx, ny := q[0]%m.w+d[0], q[0]/m.w+d[1]
				if lake(nx, ny) && !seen[nx+ny*m.w] {
					seen[nx+ny*m.w] = true
					q = append(q, nx+ny*m.w)
				}
			}
		}
		touches := false
		for _, c := range pool {
			touches = touches || mopen(m.t[c]) || mopen(m.at(c%m.w+1, c/m.w)) || mopen(m.at(c%m.w-1, c/m.w)) ||
				mopen(m.at(c%m.w, c/m.w+1)) || mopen(m.at(c%m.w, c/m.w-1))
		}
		if !touches {
			continue
		}
		for _, c := range pool {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if x, y := c%m.w+dx, c/m.w+dy; x >= 1 && y >= 1 && x < m.w-1 && y < m.h-1 && m.at(x, y) == mWall {
						m.set(x, y, mRoom) // the rim, or lava below
					}
				}
			}
		}
		for _, c := range pool {
			m.t[c] = mLava
		}
	}
	return nil
}

// posts puts a pillar, with chance p, on room floor with only room floor or walls round it (so pillars never
// touch each other, a doorway or a corridor, and stand a cell off any wall): D1's floor-tile variants with a
// pillar or a stub in them (l1Substitution, l2Substitution, l4Substitution's 95-97).
func addPosts(m *mega, p float64, rnd *rand.Rand) {
	if p <= 0 {
		return
	}
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			ok := m.at(x, y) == mRoom
			for dy := -1; dy <= 1 && ok; dy++ {
				for dx := -1; dx <= 1; dx++ {
					k := m.at(x+dx, y+dy)
					ok = ok && (k == mRoom || k == mWall)
				}
			}
			if ok && rnd.Float64() < p {
				m.set(x, y, mPost)
			}
		}
	}
}

// connect makes every walkable megatile reachable from every other: from the part holding the first one, the
// cheapest way to the next part (walking free, a ford across lava 1, a hole through a wall 3) is opened, until
// nothing is left out. It is the guarantee behind every pass above.
func connectMega(m *mega) {
	for {
		start := -1
		for i, k := range m.t {
			if mwalkable(k) {
				start = i
				break
			}
		}
		if start < 0 {
			return
		}
		cost := func(k int) int {
			switch {
			case mwalkable(k):
				return 0
			case k == mLava:
				return 1
			}
			return 3
		}
		// Dijkstra with buckets, from the start's part; the first walkable cell reached at a cost is another part
		dist, from := make([]int, len(m.t)), make([]int, len(m.t))
		for i := range dist {
			dist[i], from[i] = -1, -1
		}
		dist[start] = 0
		buckets := [][]int{{start}}
		end := -1
		for c := 0; c < len(buckets) && end < 0; c++ {
			for j := 0; j < len(buckets[c]) && end < 0; j++ {
				i := buckets[c][j]
				if dist[i] != c {
					continue
				}
				if c > 0 && mwalkable(m.t[i]) {
					end = i
					break
				}
				for _, d := range mdirs {
					x, y := i%m.w+d[0], i/m.w+d[1]
					if x < 0 || y < 0 || x >= m.w || y >= m.h || m.t[x+y*m.w] == mLocked {
						continue
					}
					n := x + y*m.w
					if nd := c + cost(m.t[n]); dist[n] < 0 || nd < dist[n] {
						dist[n], from[n] = nd, i
						for len(buckets) <= nd {
							buckets = append(buckets, nil)
						}
						buckets[nd] = append(buckets[nd], n)
					}
				}
			}
		}
		if end < 0 {
			return
		}
		for i := from[end]; i >= 0 && !mwalkable(m.t[i]); i = from[i] {
			if m.t[i] == mLava {
				m.t[i] = mRoom // a ford
			} else {
				m.t[i] = mCorr
			}
		}
	}
}

// addDoors makes each doorway a door with chance p and an open gap otherwise: the layout's own doors, and each
// corridor cell between walls on two opposite sides and floor on the other two, one of them a room's.
func addDoors(m *mega, p float64, rnd *rand.Rand) {
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			switch m.at(x, y) {
			case mDoor:
				if rnd.Float64() >= p {
					m.set(x, y, mCorr)
				}
				continue
			case mCorr:
			default:
				continue
			}
			n, s, w, e := m.at(x, y-1), m.at(x, y+1), m.at(x-1, y), m.at(x+1, y)
			ns := n == mWall && s == mWall && mopen(w) && mopen(e) && (w == mRoom || e == mRoom)
			we := w == mWall && e == mWall && mopen(n) && mopen(s) && (n == mRoom || s == mRoom)
			if (ns || we) && rnd.Float64() < p {
				m.set(x, y, mDoor)
			}
		}
	}
}

// ThemeRooms is D1's DRLG_PlaceThemeRooms on the grid: every Freq-th (0: every) room cell with an open square of
// Min..Max room cells from it gets a walled room inside that square, with a door (Doors) or a gap in its east or
// south wall. RandomSize shrinks each to a random size first, as D1's hell does.
type ThemeRooms struct {
	Min, Max, Freq int
	RandomSize     bool
	Doors          bool
}

// themeRooms ports DRLG_PlaceThemeRooms, GetSizeForThemeRoom and CreateThemeRoom's walls, and returns the rooms
// (x, y, w, h: the walls included) so the population can find them.
func themeRooms(m *mega, t ThemeRooms, rnd *rand.Rand) [][4]int {
	floor := func(k int) bool { return k == mRoom || k == mCorr } // D1's floor megatile: rooms and halls alike
	var out [][4]int
	near := func(x, y int) bool {
		for _, r := range out {
			if x >= r[0]-2 && x < r[0]+r[2]+3 && y >= r[1]-2 && y < r[1]+r[3]+3 {
				return true
			}
		}
		return false
	}
	size := func(x, y int) (int, int, bool) {
		if near(x, y) {
			return 0, 0, false
		}
		maxW, maxH := min(t.Max, m.w-x), min(t.Max, m.h-y)
		rw, rh := maxW, maxH
		for i := 0; i < t.Max; i++ {
			w, h := 0, 0
			if i < rh {
				w = i
			}
			if i < maxH {
				for w < rw && floor(m.at(x+w, y+i)) {
					w++
				}
			}
			if i < rw {
				h = i
			}
			if i < maxW {
				for h < rh && floor(m.at(x+i, y+h)) {
					h++
				}
			}
			if w < t.Min || h < t.Min {
				if i < t.Min {
					return 0, 0, false
				}
				break
			}
			rw, rh = min(rw, w), min(rh, h)
		}
		return rw - 2, rh - 2, true
	}
	for y := 0; y < m.h; y++ {
		for x := 0; x < m.w; x++ {
			if !floor(m.at(x, y)) || t.Freq > 0 && rnd.Intn(t.Freq) != 0 {
				continue
			}
			w, h, ok := size(x, y)
			if !ok {
				continue
			}
			if t.RandomSize {
				lo, hi := t.Min-2, t.Max-2
				r := func(n int) int { // D1's random_(n), 0 for n <= 0
					if n <= 0 {
						return 0
					}
					return rnd.Intn(n)
				}
				if w = lo + r(r(w-lo+1)); w > hi {
					w = lo
				}
				if h = lo + r(r(h-lo+1)); h > hi {
					h = lo
				}
			}
			lx, ly, hx, hy := x+1, y+1, x+1+w, y+1+h
			for yy := ly; yy < hy; yy++ {
				for xx := lx; xx < hx; xx++ {
					if yy == ly || yy == hy-1 || xx == lx || xx == hx-1 {
						m.set(xx, yy, mWall)
					}
				}
			}
			way := mCorr
			if t.Doors {
				way = mDoor
			}
			if rnd.Intn(2) == 0 {
				m.set(hx-1, (ly+hy)/2, way)
			} else {
				m.set((lx+hx)/2, hy-1, way)
			}
			out = append(out, [4]int{lx, ly, w, h})
		}
	}
	return out
}

// openHalls clears h[0] rectangles of h[1]..h[2] cells a side to room floor, each centred on floor it opens
// out: the open halls D1's caves have among their passages (l3CreateBlock's blocks), room for a theme room.
func openHalls(m *mega, h [3]int, rnd *rand.Rand) {
	for n := 0; n < h[0]; n++ {
		w, d := h[1]+rnd.Intn(h[2]-h[1]+1), h[1]+rnd.Intn(h[2]-h[1]+1)
		for try := 0; try < 100; try++ {
			x, y := 1+rnd.Intn(max(m.w-w-1, 1)), 1+rnd.Intn(max(m.h-d-1, 1))
			if !mopen(m.at(x+w/2, y+d/2)) {
				continue
			}
			for j := y; j < y+d; j++ {
				for i := x; i < x+w; i++ {
					m.set(i, j, mRoom)
				}
			}
			break
		}
	}
}

// bare is whether no pass is switched on.
func (p Passes) bare() bool {
	return p.Grow == 0 && !p.Spine && !p.Mirror && p.ArchChance == 0 && p.LoopChance == 0 && p.Lava == nil &&
		p.Posts == 0 && p.DoorChance < 0 && p.Canvas == 0 && p.Themes == nil && len(p.Pieces) == 0 && p.Halls[0] == 0
}
