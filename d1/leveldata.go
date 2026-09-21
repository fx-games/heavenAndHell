package d1

import (
	"encoding/binary"
	"fmt"
	"io/fs"
	"path"
	"strconv"
	"strings"
)

// Level-building data lives in assets/data/levels, imported from Diablo's DIABDAT.MPQ by cmd/d1import: a .txt
// per set piece (D1's .DUN) and til/sol tables per level type. The parsed shapes match the MPQ's byte layouts
// exactly, so the generators read them the way they always did.
//
// A .txt holds sections. "[tiles] w h" and "[transparency] w h" are grids of decimal megatile numbers, one
// dungeon row per line; "[items]", "[monsters]" and "[objects]" are TSVs of the layer's non-zero cells, which are
// sparse enough that listing them beats writing out the mostly-empty grid.
//
//go:generate go run ../cmd/d1import -dest ../files/mods/d1/data/levels
type levelData struct {
	dun      map[string]dun             // "l1/sklkng1" -> set piece
	til, sol [len(levelTypeDirs)][]byte // by level type (DType*)
}

const levelDir = "levels"

var levelTypeDirs = [5]string{"", "l1", "l2", "l3", "l4"}

func loadLevelData(fsys fs.FS) (levelData, error) {
	ld := levelData{dun: map[string]dun{}}
	for lt := 1; lt < len(levelTypeDirs); lt++ {
		dir := levelTypeDirs[lt]
		base := dataPath(levelDir, dir)
		entries, err := fs.ReadDir(fsys, base)
		if err != nil {
			return ld, fmt.Errorf("level data: %w", err)
		}
		for _, e := range entries {
			name := e.Name()
			b, err := fs.ReadFile(fsys, path.Join(base, name))
			if err != nil {
				return ld, fmt.Errorf("%s/%s: %w", dir, name, err)
			}
			switch {
			case strings.HasSuffix(name, ".txt"):
				d, err := parseDunText(b)
				if err != nil {
					return ld, fmt.Errorf("%s/%s: %w", dir, name, err)
				}
				ld.dun[dir+"/"+strings.TrimSuffix(name, ".txt")] = d
			case name == "til.tsv":
				if ld.til[lt], err = parseTil(b); err != nil {
					return ld, fmt.Errorf("%s/%s: %w", dir, name, err)
				}
			case name == "sol.tsv":
				if ld.sol[lt], err = parseSol(b); err != nil {
					return ld, fmt.Errorf("%s/%s: %w", dir, name, err)
				}
			}
		}
		if ld.til[lt] == nil || ld.sol[lt] == nil {
			return ld, fmt.Errorf("level data: %s is missing til.tsv or sol.tsv", dir)
		}
	}
	return ld, nil
}

// section splits a .txt into its "[name] args" blocks, dropping comments and blank lines.
type section struct {
	name  string
	args  []int
	lines []string
}

func sections(b []byte) ([]section, error) {
	var out []section
	for n, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "[") {
			if len(out) == 0 {
				return nil, fmt.Errorf("line %d: data before any [section]", n+1)
			}
			out[len(out)-1].lines = append(out[len(out)-1].lines, line)
			continue
		}
		end := strings.IndexByte(line, ']')
		if end < 0 {
			return nil, fmt.Errorf("line %d: unterminated [section]", n+1)
		}
		s := section{name: line[1:end]}
		for _, f := range strings.Fields(line[end+1:]) {
			v, err := strconv.Atoi(f)
			if err != nil {
				return nil, fmt.Errorf("line %d: bad [%s] argument %q", n+1, s.name, f)
			}
			s.args = append(s.args, v)
		}
		out = append(out, s)
	}
	return out, nil
}

// grid reads w×h decimal words. Short rows pad with zeros, so an editor stripping trailing whitespace is harmless.
func (s section) grid() ([]uint16, int, int, error) {
	if len(s.args) != 2 {
		return nil, 0, 0, fmt.Errorf("[%s] needs a width and a height", s.name)
	}
	w, h := s.args[0], s.args[1]
	if w <= 0 || h <= 0 {
		return nil, 0, 0, fmt.Errorf("[%s] has size %dx%d", s.name, w, h)
	}
	if len(s.lines) != h {
		return nil, 0, 0, fmt.Errorf("[%s] is %dx%d but has %d rows", s.name, w, h, len(s.lines))
	}
	out := make([]uint16, w*h)
	for j, line := range s.lines {
		fields := strings.Fields(line)
		if len(fields) > w {
			return nil, 0, 0, fmt.Errorf("[%s] row %d has %d values, want %d", s.name, j, len(fields), w)
		}
		for i, f := range fields {
			v, err := strconv.ParseUint(f, 10, 16)
			if err != nil {
				return nil, 0, 0, fmt.Errorf("[%s] row %d column %d: %q", s.name, j, i, f)
			}
			out[j*w+i] = uint16(v)
		}
	}
	return out, w, h, nil
}

// cells reads an "x y id" TSV of a layer's non-zero cells.
func (s section) cells() ([][3]int, error) {
	if len(s.lines) == 0 {
		return nil, nil
	}
	if head := strings.Fields(s.lines[0]); len(head) == 0 || head[0] != "x" {
		return nil, fmt.Errorf("[%s] is missing its x/y/id header", s.name)
	}
	out := make([][3]int, 0, len(s.lines)-1)
	for n, line := range s.lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			return nil, fmt.Errorf("[%s] row %d: want x, y and id", s.name, n)
		}
		var c [3]int
		for i := 0; i < 3; i++ {
			v, err := strconv.Atoi(strings.TrimSpace(fields[i]))
			if err != nil {
				return nil, fmt.Errorf("[%s] row %d: %q", s.name, n, fields[i])
			}
			c[i] = v
		}
		out = append(out, c)
	}
	return out, nil
}

// dunLayers is the order .DUN layers sit in, after the tiles: dun.layer's n.
var dunLayers = [4]string{"items", "monsters", "objects", "transparency"}

// parseDunText builds the same dun a .DUN file does: size, tiles, then four double-resolution layers.
func parseDunText(b []byte) (dun, error) {
	secs, err := sections(b)
	if err != nil {
		return nil, err
	}
	if len(secs) == 0 || secs[0].name != "tiles" {
		return nil, fmt.Errorf("first section must be [tiles]")
	}
	tiles, w, h, err := secs[0].grid()
	if err != nil {
		return nil, err
	}
	d := make(dun, 2+w*h+4*4*w*h)
	d[0], d[1] = uint16(w), uint16(h)
	copy(d[2:], tiles)
	set := func(n, i, j, v int) error {
		if i < 0 || j < 0 || i >= 2*w || j >= 2*h {
			return fmt.Errorf("[%s] cell %d,%d is outside %dx%d", dunLayers[n], i, j, 2*w, 2*h)
		}
		d[2+w*h+n*4*w*h+j*2*w+i] = uint16(v)
		return nil
	}
	for _, s := range secs[1:] {
		n := -1
		for i, name := range dunLayers {
			if s.name == name {
				n = i
			}
		}
		switch {
		case n < 0:
			return nil, fmt.Errorf("unknown section [%s]", s.name)
		case s.name == "transparency":
			g, gw, gh, err := s.grid()
			if err != nil {
				return nil, err
			}
			if gw != 2*w || gh != 2*h {
				return nil, fmt.Errorf("[transparency] is %dx%d, want %dx%d", gw, gh, 2*w, 2*h)
			}
			for j := 0; j < gh; j++ {
				for i := 0; i < gw; i++ {
					if err := set(n, i, j, int(g[j*gw+i])); err != nil {
						return nil, err
					}
				}
			}
		default:
			cells, err := s.cells()
			if err != nil {
				return nil, err
			}
			for _, c := range cells {
				if err := set(n, c[0], c[1], c[2]); err != nil {
					return nil, err
				}
			}
		}
	}
	return d, nil
}

// tsv reads a tab-separated table with a header row, strictly: every field must be a number.
func tsv(b []byte, want []string) ([][]int, error) {
	var out [][]int
	for n, line := range strings.Split(string(b), "\n") {
		line = strings.TrimRight(line, " \t\r")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if out == nil && len(fields) > 0 && fields[0] == want[0] {
			if len(fields) != len(want) {
				return nil, fmt.Errorf("header has %d columns, want %d", len(fields), len(want))
			}
			for i, w := range want {
				if strings.TrimSpace(fields[i]) != w {
					return nil, fmt.Errorf("column %d is %q, want %q", i, fields[i], w)
				}
			}
			out = [][]int{}
			continue
		}
		if out == nil {
			return nil, fmt.Errorf("line %d: expected the header row %v", n+1, want)
		}
		if len(fields) != len(want) {
			return nil, fmt.Errorf("line %d: %d columns, want %d", n+1, len(fields), len(want))
		}
		vals := make([]int, len(fields))
		for i, f := range fields {
			f = strings.TrimSpace(f)
			base := 10
			if strings.HasPrefix(f, "0x") {
				base, f = 16, f[2:]
			}
			v, err := strconv.ParseInt(f, base, 32)
			if err != nil {
				return nil, fmt.Errorf("line %d column %d: %q", n+1, i, f)
			}
			vals[i] = int(v)
		}
		out = append(out, vals)
	}
	if out == nil {
		return nil, fmt.Errorf("no header row")
	}
	return out, nil
}

// parseTil rebuilds a .TIL: four little-endian piece numbers per megatile, in the order lPass3 lays them out.
func parseTil(b []byte) ([]byte, error) {
	rows, err := tsv(b, []string{"id", "nw", "ne", "sw", "se"})
	if err != nil {
		return nil, err
	}
	out := make([]byte, 8*len(rows))
	for n, r := range rows {
		if r[0] != n+1 {
			return nil, fmt.Errorf("row %d has id %d, want %d", n, r[0], n+1)
		}
		for i := 0; i < 4; i++ {
			if r[1+i] < 0 || r[1+i] > 0xFFFF {
				return nil, fmt.Errorf("megatile %d: piece %d out of range", r[0], r[1+i])
			}
			binary.LittleEndian.PutUint16(out[8*n+2*i:], uint16(r[1+i]))
		}
	}
	return out, nil
}

// parseSol rebuilds a .SOL: one flag byte per piece.
func parseSol(b []byte) ([]byte, error) {
	rows, err := tsv(b, []string{"piece", "flags"})
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(rows))
	for n, r := range rows {
		if r[0] != n {
			return nil, fmt.Errorf("row %d has piece %d, want %d", n, r[0], n)
		}
		if r[1] < 0 || r[1] > 0xFF {
			return nil, fmt.Errorf("piece %d: flags %d out of range", r[0], r[1])
		}
		out[n] = byte(r[1])
	}
	return out, nil
}
