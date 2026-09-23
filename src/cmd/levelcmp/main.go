// Command levelcmp shows D1's level generators next to roguelike stand-ins built from package dungen's layout
// generators, one level at a time and at the same scale (one character = one D1 map cell, 80x80), so the two can
// be compared by eye before D1's generators are replaced.
//
//	go run ./src/cmd/levelcmp -d1              interactive: 1-4 level type, same key or space rerolls, q quits
//	go run ./src/cmd/levelcmp -alt
//	go run ./src/cmd/levelcmp -alt -type 3 -seed 42   print one level and exit
//
// # wall   . floor   + door   ~ water/lava   (blank: rock no floor touches)
package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"

	"golang.org/x/term"

	"rpg/files"
	"rpg/src/defs"
	"rpg/src/dungen"
	"rpg/src/rules"
)

var typeNames = [5]string{"", "cathedral", "catacombs", "caves", "hell"}

// d1Dlvl is the dungeon level -d1 builds for each type: the second of each four, which holds no set piece that
// fills the map (quests are rolled from the seed as in a new game, so a quest room can still turn up).
var d1Dlvl = [5]int{0, 2, 6, 10, 14}

// cell is what the printout shows at one map cell.
type cell byte

const (
	cRock  cell = ' '
	cWall  cell = '#'
	cFloor cell = '.'
	cDoor  cell = '+'
	cWater cell = '~'
)

// grid is a level as the printout sees it: solid or not, door, water/lava. Walls no floor touches print as rock.
type grid struct {
	w, h int
	c    []cell
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, c: make([]cell, w*h)}
	for i := range g.c {
		g.c[i] = cWall
	}
	return g
}

func (g *grid) at(x, y int) cell {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return cWall
	}
	return g.c[x+y*g.w]
}

func (g *grid) set(x, y int, c cell) { g.c[x+y*g.w] = c }

func (g *grid) render(header string) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			c := g.at(x, y)
			if c == cWall && !g.touchesOpen(x, y) {
				c = cRock
			}
			b.WriteByte(byte(c))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func (g *grid) touchesOpen(x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if c := g.at(x+dx, y+dy); c != cWall && c != cRock {
				return true
			}
		}
	}
	return false
}

func (g *grid) walkable(x, y int) bool { c := g.at(x, y); return c == cFloor || c == cDoor }

// posts counts pillars and wall stubs: solid cells with floor on three or four sides (arcades count once a pillar).
func (g *grid) posts() int {
	n := 0
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if g.at(x, y) != cWall {
				continue
			}
			f := 0
			for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
				if g.walkable(x+d[0], y+d[1]) {
					f++
				}
			}
			if f >= 3 {
				n++
			}
		}
	}
	return n
}

// loops counts the ways round: islands of wall, rock or water (8-connected, at least 6 cells, so not pillars)
// that walkable ground encloses without touching the map's edge. On a connected level that is its number of
// independent cycles, the openings beyond a spanning tree.
func (g *grid) loops() int {
	seen := make([]bool, len(g.c))
	n := 0
	for i := range g.c {
		if seen[i] || g.walkable(i%g.w, i/g.w) {
			continue
		}
		size, edge := 0, false
		seen[i] = true
		for q := []int{i}; len(q) > 0; q = q[1:] {
			x, y := q[0]%g.w, q[0]/g.w
			size++
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nx, ny := x+dx, y+dy
					if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
						edge = true
						continue
					}
					if j := nx + ny*g.w; !seen[j] && !g.walkable(nx, ny) {
						seen[j] = true
						q = append(q, j)
					}
				}
			}
		}
		if !edge && size >= 6 {
			n++
		}
	}
	return n
}

// unreachable counts the walkable cells the first one cannot reach (4-connected).
func (g *grid) unreachable() int {
	total, start := 0, -1
	for i := range g.c {
		if g.walkable(i%g.w, i/g.w) {
			total++
			if start < 0 {
				start = i
			}
		}
	}
	if start < 0 {
		return 0
	}
	seen := map[int]bool{start: true}
	for q := []int{start}; len(q) > 0; q = q[1:] {
		x, y := q[0]%g.w, q[0]/g.w
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			if nx, ny := x+d[0], y+d[1]; g.walkable(nx, ny) && !seen[nx+ny*g.w] {
				seen[nx+ny*g.w] = true
				q = append(q, nx+ny*g.w)
			}
		}
	}
	return total - len(seen)
}

func (g *grid) floorPct() float64 {
	n := 0
	for _, c := range g.c {
		if c == cFloor || c == cDoor {
			n++
		}
	}
	return 100 * float64(n) / float64(len(g.c))
}

// ---- D1 ----

type d1 struct {
	data   *rules.Data
	levels *dungen.Levels
}

func loadD1() *d1 {
	files.SelectMod("d1")
	if err := defs.LoadIcons(); err != nil { // defs.IconLava: L3's lava pieces are drawn with it
		log.Fatal(err)
	}
	data, err := rules.Load(files.GetFileSystem())
	if err != nil {
		log.Fatal(err)
	}
	levels, err := dungen.LoadLevels(files.GetFileSystem())
	if err != nil {
		log.Fatal(err)
	}
	return &d1{data: data, levels: levels}
}

// build is src/app's maptest (dun_gen) path: a new game's seeds and quests, then the level.
func (d *d1) build(ltype int, seed int64) (*grid, error) {
	seeds := rules.LevelSeeds(int32(seed))
	quests := d.data.InitQuests(seeds)
	gen := rules.NewGenerator(d.data, int32(seed))
	lvl, err := d.levels.CreateLevel(gen, &quests, seeds, d1Dlvl[ltype], rules.SLNone, dungen.EntryMain)
	if err != nil {
		return nil, err
	}
	g := newGrid(dungen.LevelSize, dungen.LevelSize)
	doors := map[[2]int]bool{} // the door objects the game puts on its door pieces (common/map_gen.go)
	for _, o := range lvl.Objects {
		switch o.Kind {
		case rules.ObjL1LDOOR, rules.ObjL1RDOOR, rules.ObjL2LDOOR, rules.ObjL2RDOOR, rules.ObjL3LDOOR, rules.ObjL3RDOOR:
			doors[[2]int{o.X - dungen.CellOrigin, o.Y - dungen.CellOrigin}] = true
		}
	}
	for y := 0; y < dungen.LevelSize; y++ {
		for x := 0; x < dungen.LevelSize; x++ {
			f := lvl.CellFlags(x, y)
			switch {
			case doors[[2]int{x, y}] || lvl.CellDoor(x, y):
				g.set(x, y, cDoor)
			case f.Water || f.Damaging || lvl.CellTile(x, y) == int(defs.IconLava):
				g.set(x, y, cWater)
			case !lvl.CellSolid(x, y):
				g.set(x, y, cFloor)
			}
		}
	}
	return g, nil
}

// ---- main loop ----

func main() {
	useD1 := flag.Bool("d1", false, "show D1's own generators")
	useAlt := flag.Bool("alt", false, "show the roguelike stand-ins")
	once := flag.Int("type", 0, "print level type 1-4 once and exit")
	seedFlag := flag.Int64("seed", 0, "seed for -type (0: random)")
	flag.Parse()
	if *useD1 == *useAlt {
		fmt.Fprintln(os.Stderr, "levelcmp: pass exactly one of -d1 or -alt")
		os.Exit(2)
	}
	mode := "alt"
	var build func(ltype int, seed int64) (*grid, error) = buildAlt
	if *useD1 {
		mode, build = "d1", loadD1().build
	}
	show := func(ltype int, seed int64, nl string) {
		g, err := build(ltype, seed)
		if err != nil {
			fmt.Printf("%s %s seed %d: %v%s", mode, typeNames[ltype], seed, err, nl)
			return
		}
		out := g.render(fmt.Sprintf("%s  %s  seed %d  floor %.1f%%  posts %d  loops %d   [1-4 type, space reroll, q quit]",
			mode, typeNames[ltype], seed, g.floorPct(), g.posts(), g.loops()))
		fmt.Print(strings.ReplaceAll(out, "\n", nl))
	}
	if *once != 0 {
		if *once < 1 || *once > 4 {
			log.Fatal("levelcmp: -type must be 1..4")
		}
		seed := *seedFlag
		if seed == 0 {
			seed = rand.Int63n(1 << 31)
		}
		show(*once, seed, "\n")
		return
	}

	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatal(err)
	}
	defer term.Restore(fd, old)
	ltype := 1
	key := make([]byte, 1)
	for {
		fmt.Print("\033[H\033[2J")
		show(ltype, rand.Int63n(1<<31), "\r\n")
		if _, err := os.Stdin.Read(key); err != nil {
			return
		}
		switch k := key[0]; {
		case k == 'q' || k == 3: // q or ctrl-c
			return
		case k >= '1' && k <= '4':
			ltype = int(k - '0')
		}
	}
}
