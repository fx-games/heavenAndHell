package main

// Diablo's set pieces: the .txt files the game's megatile generators stamp into a level, and the til/sol
// tables that say what a megatile is made of and what those pieces block. The game reads these files; this is
// the writer that keeps them exactly as it expects them.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// A set piece is w*h megatiles, with four layers laid over it at twice that resolution.
type setPiece struct {
	path       string
	levelType  string // l1..l4, which til/sol tables it uses
	w, h       int
	tiles      []int // w*h megatile numbers, 0 for nothing
	trans      []int // 2w*2h transparency regions, 0 for none
	layers     map[string]map[point]int
	headerNote string // the file's own first comment line, kept as it was
}

type point struct{ x, y int }

// dunLayers are the sparse double-resolution layers, in the order a file writes them.
var dunLayers = []string{"items", "monsters", "objects"}

func newSetPiece(path, levelType string, w, h int) *setPiece {
	p := &setPiece{path: path, levelType: levelType, w: w, h: h,
		tiles: make([]int, w*h), trans: make([]int, 4*w*h), layers: map[string]map[point]int{}}
	for _, n := range dunLayers {
		p.layers[n] = map[point]int{}
	}
	return p
}

func (p *setPiece) name() string { return strings.TrimSuffix(filepath.Base(p.path), ".txt") }

func (p *setPiece) tileAt(x, y int) int {
	if x < 0 || y < 0 || x >= p.w || y >= p.h {
		return -1
	}
	return p.tiles[y*p.w+x]
}

func (p *setPiece) setTile(x, y, v int) {
	if x >= 0 && y >= 0 && x < p.w && y < p.h {
		p.tiles[y*p.w+x] = v
	}
}

func (p *setPiece) transAt(x, y int) int {
	if x < 0 || y < 0 || x >= 2*p.w || y >= 2*p.h {
		return -1
	}
	return p.trans[y*2*p.w+x]
}

func (p *setPiece) setTrans(x, y, v int) {
	if x >= 0 && y >= 0 && x < 2*p.w && y < 2*p.h {
		p.trans[y*2*p.w+x] = v
	}
}

// resize grows or crops the piece, keeping what still fits.
func (p *setPiece) resize(w, h int) {
	q := newSetPiece(p.path, p.levelType, w, h)
	q.headerNote = p.headerNote
	for y := 0; y < min(h, p.h); y++ {
		for x := 0; x < min(w, p.w); x++ {
			q.setTile(x, y, p.tileAt(x, y))
		}
	}
	for y := 0; y < min(2*h, 2*p.h); y++ {
		for x := 0; x < min(2*w, 2*p.w); x++ {
			q.setTrans(x, y, p.transAt(x, y))
		}
	}
	for _, n := range dunLayers {
		for pt, v := range p.layers[n] {
			if pt.x < 2*w && pt.y < 2*h {
				q.layers[n][pt] = v
			}
		}
	}
	*p = *q
}

func (p *setPiece) clone() *setPiece {
	q := newSetPiece(p.path, p.levelType, p.w, p.h)
	q.headerNote = p.headerNote
	copy(q.tiles, p.tiles)
	copy(q.trans, p.trans)
	for _, n := range dunLayers {
		for pt, v := range p.layers[n] {
			q.layers[n][pt] = v
		}
	}
	return q
}

// readSetPiece parses one .txt, the same sections the game's own reader accepts.
func readSetPiece(path, levelType string) (*setPiece, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p *setPiece
	sec, args, row := "", []int{}, 0
	note := ""
	for n, line := range strings.Split(string(b), "\n") {
		raw := strings.TrimRight(line, " \t\r")
		if strings.HasPrefix(raw, "#") {
			if note == "" {
				note = strings.TrimSpace(raw[1:])
			}
			continue
		}
		if raw == "" {
			continue
		}
		if strings.HasPrefix(raw, "[") {
			end := strings.IndexByte(raw, ']')
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated [section]", n+1)
			}
			sec, row = raw[1:end], 0
			args = args[:0]
			for _, f := range strings.Fields(raw[end+1:]) {
				v, err := strconv.Atoi(f)
				if err != nil {
					return nil, fmt.Errorf("line %d: bad [%s] argument %q", n+1, sec, f)
				}
				args = append(args, v)
			}
			if sec == "tiles" {
				if len(args) != 2 || args[0] <= 0 || args[1] <= 0 {
					return nil, fmt.Errorf("line %d: [tiles] needs a positive width and height", n+1)
				}
				p = newSetPiece(path, levelType, args[0], args[1])
				p.headerNote = note
			}
			continue
		}
		if p == nil {
			return nil, fmt.Errorf("line %d: data before [tiles]", n+1)
		}
		switch sec {
		case "tiles", "transparency":
			for x, f := range strings.Fields(raw) {
				v, err := strconv.Atoi(f)
				if err != nil {
					return nil, fmt.Errorf("line %d: %q", n+1, f)
				}
				if sec == "tiles" {
					p.setTile(x, row, v)
				} else {
					p.setTrans(x, row, v)
				}
			}
			row++
		default:
			l, ok := p.layers[sec]
			if !ok {
				return nil, fmt.Errorf("line %d: unknown section [%s]", n+1, sec)
			}
			f := strings.Fields(raw)
			if len(f) < 3 {
				return nil, fmt.Errorf("line %d: [%s] wants x, y and id", n+1, sec)
			}
			if f[0] == "x" {
				continue // the header row
			}
			x, y, id := atoiOr(f[0]), atoiOr(f[1]), atoiOr(f[2])
			l[point{x, y}] = id
		}
	}
	if p == nil {
		return nil, fmt.Errorf("%s has no [tiles] section", path)
	}
	return p, nil
}

func atoiOr(s string) int { v, _ := strconv.Atoi(strings.TrimSpace(s)); return v }

// write renders the .txt, in the layout the shipped files already use.
func (p *setPiece) write() []byte {
	var b strings.Builder
	note := p.headerNote
	if note == "" {
		note = fmt.Sprintf("%s  %dx%d megatiles", p.name(), p.w, p.h)
	}
	fmt.Fprintf(&b, "# %s\n[tiles] %d %d\n", note, p.w, p.h)
	writeGrid(&b, p.tiles, p.w, p.h)
	for _, n := range dunLayers {
		l := p.layers[n]
		if len(l) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n[%s]\nx\ty\tid\n", n)
		keys := make([]point, 0, len(l))
		for k := range l {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].y != keys[j].y {
				return keys[i].y < keys[j].y
			}
			return keys[i].x < keys[j].x
		})
		for _, k := range keys {
			fmt.Fprintf(&b, "%d\t%d\t%d\n", k.x, k.y, l[k])
		}
	}
	for _, v := range p.trans {
		if v != 0 {
			fmt.Fprintf(&b, "\n[transparency] %d %d\n", 2*p.w, 2*p.h)
			writeGrid(&b, p.trans, 2*p.w, 2*p.h)
			break
		}
	}
	return []byte(b.String())
}

func writeGrid(b *strings.Builder, g []int, w, h int) {
	width := 1
	for _, v := range g {
		width = max(width, len(strconv.Itoa(v)))
	}
	row := make([]string, w)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			row[x] = fmt.Sprintf("%*d", width, g[y*w+x])
		}
		b.WriteString(strings.Join(row, " ") + "\n")
	}
}

func (p *setPiece) save() error { return os.WriteFile(p.path, p.write(), 0o644) }

// megaTable is one level type's til.tsv and sol.tsv: what each megatile is made of, and what blocks.
type megaTable struct {
	pieces [][4]int // by megatile number - 1
	sol    []byte   // by piece number
}

// flags of a piece: 1 solid, 2 blocks missiles and sight, 4 blocks missiles only.
func (t *megaTable) pieceFlags(piece int) byte {
	if piece < 0 || piece >= len(t.sol) {
		return 0
	}
	return t.sol[piece]
}

// quarters returns the megatile's four pieces, in the order the generator lays them out: nw, ne, sw, se.
func (t *megaTable) quarters(mega int) [4]int {
	if mega-1 < 0 || mega-1 >= len(t.pieces) {
		return [4]int{}
	}
	return t.pieces[mega-1]
}

func (t *megaTable) count() int { return len(t.pieces) }

// readMegaTable reads a level type's til.tsv and sol.tsv out of its directory.
func readMegaTable(dir string) (*megaTable, error) {
	til, err := readTSV(filepath.Join(dir, "til.tsv"), 5)
	if err != nil {
		return nil, err
	}
	sol, err := readTSV(filepath.Join(dir, "sol.tsv"), 2)
	if err != nil {
		return nil, err
	}
	t := &megaTable{}
	for _, r := range til {
		t.pieces = append(t.pieces, [4]int{r[1], r[2], r[3], r[4]})
	}
	for _, r := range sol {
		t.sol = append(t.sol, byte(r[1]))
	}
	return t, nil
}

// readTSV reads a numeric table with a header row.
func readTSV(path string, cols int) ([][]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out [][]int
	for n, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != cols {
			return nil, fmt.Errorf("%s line %d: %d columns, want %d", path, n+1, len(f), cols)
		}
		if _, err := strconv.Atoi(strings.TrimSpace(f[0])); err != nil {
			continue // the header row
		}
		vals := make([]int, cols)
		for i, s := range f {
			s = strings.TrimSpace(s)
			base := 10
			if strings.HasPrefix(s, "0x") {
				base, s = 16, s[2:]
			}
			v, err := strconv.ParseInt(s, base, 32)
			if err != nil {
				return nil, fmt.Errorf("%s line %d column %d: %q", path, n+1, i, s)
			}
			vals[i] = int(v)
		}
		out = append(out, vals)
	}
	return out, nil
}
