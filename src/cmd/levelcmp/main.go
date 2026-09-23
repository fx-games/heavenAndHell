// Command levelcmp shows the d1 mod's dungeon levels as levels.rec lays them out (layout and passes, set pieces
// left out), one level at a time, one character a map cell, for tuning the knobs by eye.
//
//	go run ./src/cmd/levelcmp                   interactive: 1-4 level type, same key or space rerolls, q quits
//	go run ./src/cmd/levelcmp -type 3 -seed 42  print one level and exit
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
)

var typeNames = [5]string{"", "cathedral", "catacombs", "caves", "hell"}

// d1Dlvl is the dungeon level shown for each type: the second of each four.
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

// ---- main loop ----

func main() {
	once := flag.Int("type", 0, "print level type 1-4 once and exit")
	seedFlag := flag.Int64("seed", 0, "seed for -type (0: random)")
	flag.Parse()
	mode, build := "d1", buildAlt
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
