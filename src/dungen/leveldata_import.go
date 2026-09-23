package dungen

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// ImportLevels converts Diablo's level data into the text files under mods/d1/data/levels. read pulls a member
// out of a DIABDAT.MPQ (cmd/d1import wires it up); destDir is the levels directory to write.
//
// Every file is parsed back and compared to the bytes it came from before it is written, so an encoding slip
// fails the import instead of quietly changing a dungeon.
func ImportLevels(read func(name string) ([]byte, error), destDir string) error {
	for lt := 1; lt < len(levelTypeDirs); lt++ {
		dir := filepath.Join(destDir, levelTypeDirs[lt])
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		til, err := read(tilFiles[lt])
		if err != nil {
			return err
		}
		if err := writeChecked(filepath.Join(dir, "til.tsv"), renderTil(til), til, parseTil); err != nil {
			return err
		}
		for _, name := range importDuns[lt] {
			b, err := read(fmt.Sprintf(`Levels\L%dData\%s.DUN`, lt, name))
			if err != nil {
				return err
			}
			src := binDun(b)
			text := renderDun(strings.ToLower(name), src)
			got, err := parseDunText([]byte(text))
			if err != nil {
				return fmt.Errorf("%s: reparsing what we wrote: %w", name, err)
			}
			if err := sameDun(src, got); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			if err := os.WriteFile(filepath.Join(dir, strings.ToLower(name)+".txt"), []byte(text), 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// importDuns lists the set pieces the generators load, by level type.
var importDuns = [5][]string{
	1: {"Banner1", "Banner2", "SKngDO", "SklKng1", "SklKng2", "Vile1", "Vile2", "rnd6"},
	2: {"Blind1", "Blind2", "Blood1", "Blood2", "Bonecha1", "Bonecha2", "Bonestr1", "Bonestr2"},
	3: {"Anvil", "Foulwatr"},
	4: {"Warlord", "Warlord2", "diab1", "diab2a", "diab2b", "diab3a", "diab3b", "diab4a", "diab4b"},
}

// binDun reads a .DUN file into the dun layout.
func binDun(b []byte) dun {
	d := make(dun, len(b)/2)
	for i := range d {
		d[i] = binary.LittleEndian.Uint16(b[2*i:])
	}
	return d
}

func writeChecked(path, text string, want []byte, parse func([]byte) ([]byte, error)) error {
	got, err := parse([]byte(text))
	if err != nil {
		return fmt.Errorf("%s: reparsing what we wrote: %w", path, err)
	}
	if len(got) != len(want) {
		return fmt.Errorf("%s: round trip produced %d bytes, want %d", path, len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Errorf("%s: round trip differs at byte %d: %d vs %d", path, i, got[i], want[i])
		}
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

// sameDun compares two set pieces through the accessors the generators use, so a layer the .DUN leaves off and a
// layer written out as all zeros count as equal.
func sameDun(want, got dun) error {
	if want.w() != got.w() || want.h() != got.h() {
		return fmt.Errorf("round trip is %dx%d, want %dx%d", got.w(), got.h(), want.w(), want.h())
	}
	for j := 0; j < want.h(); j++ {
		for i := 0; i < want.w(); i++ {
			if want.tile(i, j) != got.tile(i, j) {
				return fmt.Errorf("round trip tile %d,%d is %d, want %d", i, j, got.tile(i, j), want.tile(i, j))
			}
		}
	}
	for n := 0; n < 4; n++ {
		for j := 0; j < 2*want.h(); j++ {
			for i := 0; i < 2*want.w(); i++ {
				if want.layer(n, i, j) != got.layer(n, i, j) {
					return fmt.Errorf("round trip %s %d,%d is %d, want %d",
						dunLayers[n], i, j, got.layer(n, i, j), want.layer(n, i, j))
				}
			}
		}
	}
	return nil
}

// renderGrid writes w×h decimal words, right-aligned so the columns line up.
func renderGrid(sb *strings.Builder, name string, w, h int, at func(i, j int) int) {
	width := 1
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			if l := len(strconv.Itoa(at(i, j))); l > width {
				width = l
			}
		}
	}
	fmt.Fprintf(sb, "[%s] %d %d\n", name, w, h)
	for j := 0; j < h; j++ {
		for i := 0; i < w; i++ {
			if i > 0 {
				sb.WriteByte(' ')
			}
			fmt.Fprintf(sb, "%*d", width, at(i, j))
		}
		sb.WriteByte('\n')
	}
}

func renderDun(name string, d dun) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s  %dx%d megatiles\n", name, d.w(), d.h())
	renderGrid(&sb, "tiles", d.w(), d.h(), func(i, j int) int { return int(d.tile(i, j)) })
	for n := 0; n < 3; n++ {
		type cell struct{ x, y, v int }
		var cells []cell
		for j := 0; j < 2*d.h(); j++ {
			for i := 0; i < 2*d.w(); i++ {
				if v := d.layer(n, i, j); v != 0 {
					cells = append(cells, cell{i, j, v})
				}
			}
		}
		if len(cells) == 0 {
			continue
		}
		sort.Slice(cells, func(a, b int) bool {
			if cells[a].y != cells[b].y {
				return cells[a].y < cells[b].y
			}
			return cells[a].x < cells[b].x
		})
		fmt.Fprintf(&sb, "\n[%s]\nx\ty\tid\n", dunLayers[n])
		for _, c := range cells {
			fmt.Fprintf(&sb, "%d\t%d\t%d\n", c.x, c.y, c.v)
		}
	}
	empty := true
	for j := 0; j < 2*d.h() && empty; j++ {
		for i := 0; i < 2*d.w(); i++ {
			if d.layer(3, i, j) != 0 {
				empty = false
				break
			}
		}
	}
	if !empty {
		sb.WriteByte('\n')
		renderGrid(&sb, "transparency", 2*d.w(), 2*d.h(), func(i, j int) int { return d.layer(3, i, j) })
	}
	return sb.String()
}

func renderTil(til []byte) string {
	var sb strings.Builder
	sb.WriteString("id\tnw\tne\tsw\tse\n")
	for i := 0; i+8 <= len(til); i += 8 {
		fmt.Fprintf(&sb, "%d\t%d\t%d\t%d\t%d\n", i/8+1,
			binary.LittleEndian.Uint16(til[i:]), binary.LittleEndian.Uint16(til[i+2:]),
			binary.LittleEndian.Uint16(til[i+4:]), binary.LittleEndian.Uint16(til[i+6:]))
	}
	return sb.String()
}
