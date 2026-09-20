package main

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"

	"rpg/defs"
)

// tiles shows every object graphic (the props' icons) and every megatile of each level type.
func tiles(read reader, _ []byte) ([]section, error) {
	sections, err := objects(read)
	if err != nil {
		return nil, err
	}
	for _, l := range []struct {
		name, path, pal string
		blocks          int // micro tiles per piece, two to a row
	}{
		{"town", `NLevels\TownData\Town`, `Levels\TownData\Town.pal`, 16},
		{"cathedral", `Levels\L1Data\L1`, `Levels\L1Data\L1_1.PAL`, 10},
		{"catacombs", `Levels\L2Data\L2`, `Levels\L2Data\L2_1.PAL`, 10},
		{"caves", `Levels\L3Data\L3`, `Levels\L3Data\L3_1.PAL`, 10},
		{"hell", `Levels\L4Data\L4`, `Levels\L4Data\L4_1.PAL`, 16},
		{"crypt", `NLevels\L5Data\L5`, `NLevels\L5Data\L5Base.PAL`, 10},
		{"nest", `NLevels\L6Data\L6`, `NLevels\L6Data\L6Base1.PAL`, 10},
	} {
		s, err := megatiles(read, l.name, l.path, l.pal, l.blocks)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", l.name, err)
		}
		sections = append(sections, s)
	}
	return sections, nil
}

// objects shows each object CEL: every frame of a still one (closed, open, broken...), the first of an animated
// one, with the props drawn from it and their current icons.
func objects(read reader) ([]section, error) {
	icons := loadIcons("tiles")
	pals := map[int][]byte{}
	var sections []section
	byFile := map[string]int{}
	placeholders := section{title: "Objects without graphics (no width; their ofindex is a placeholder)"}
	for _, r := range table("xl_obj") {
		file := strings.TrimPrefix(r.s("ofindex"), "OFILE_")
		if file == "0" {
			continue
		}
		key := defs.PropIconID(r.s(""))
		note := fmt.Sprintf("%s: icon %s", key, icons.icon(key))
		if icons.has(key + ".used") {
			note += ", used " + icons.icon(key+".used")
		}
		if r.i("oAnimWidth") == 0 {
			placeholders.notes = append(placeholders.notes, note)
			continue
		}
		// ponytail: frame 1 is the fresh prop, frame 2 (else 1) the used one; D1's per-object used frames if it matters
		addKeys := func(cs []card) {
			cs[0].keys = append(cs[0].keys, key)
			cs[min(1, len(cs)-1)].keys = append(cs[min(1, len(cs)-1)].keys, key+".used")
		}
		if i, ok := byFile[file]; ok {
			sections[i].notes = append(sections[i].notes, note)
			addKeys(sections[i].cards)
			continue
		}
		lvl := max(r.i("olvltype"), 1)
		if pals[lvl] == nil {
			p, err := read(fmt.Sprintf(`Levels\L%dData\L%d_1.PAL`, lvl, lvl))
			if err != nil {
				return nil, err
			}
			pals[lvl] = p
		}
		cel, err := read(`Objects\` + file + ".CEL")
		if err != nil {
			return nil, fmt.Errorf("%s: %v", file, err)
		}
		frames := int(le.Uint32(cel))
		if r.i("oAnimFlag") == 1 {
			frames = 1
		}
		s := section{title: "Objects\\" + file + ".CEL", notes: []string{note}}
		for n := range frames {
			f, err := celFrame(cel, n, true)
			if err != nil {
				return nil, fmt.Errorf("%s: %v", file, err)
			}
			s.cards = append(s.cards, card{img: crop(render(decodeCel(f), r.i("oAnimWidth"), pals[lvl])),
				file: fmt.Sprintf("objects/%s.frame%02d.png", strings.ToLower(file), n+1), lines: []string{fmt.Sprintf("frame %d", n+1)}})
		}
		addKeys(s.cards)
		byFile[file] = len(sections)
		sections = append(sections, s)
	}
	return append(sections, placeholders), nil
}

// megatiles renders each TIL entry the way the dungeon draws it: four pieces on the isometric grid, each piece a
// column of 32x32 micro tiles from the level CEL, as the MIN lists them (top row first).
func megatiles(read reader, name, path, palPath string, blocks int) (section, error) {
	s := section{title: name + " megatiles (" + path + ".TIL)"}
	var files [5][]byte
	for i, ext := range []string{".CEL", ".MIN", ".TIL", ".SOL", ""} {
		var err error
		if ext == "" {
			files[i], err = read(palPath)
		} else if files[i], err = read(path + ext); err != nil && strings.HasPrefix(path, `NLevels\TownData`) {
			files[i], err = read(`Levels` + strings.TrimPrefix(path, `NLevels`) + ext) // Diablo without the expansion
		}
		if err != nil {
			return s, err
		}
	}
	cel, min, til, sol, pal := files[0], files[1], files[2], files[3], brighten(files[4])
	pieceH := blocks / 2 * 32
	piece := func(id int) (*image.NRGBA, error) {
		img := image.NewNRGBA(image.Rect(0, 0, 64, pieceH))
		for b := range blocks {
			v := le.Uint16(min[(id*blocks+b)*2:])
			if v == 0 {
				continue
			}
			f, err := celFrame(cel, int(v&0xFFF)-1, false)
			if err != nil {
				return nil, fmt.Errorf("piece %d: %v", id, err)
			}
			micro := render(decodeMicro(f, int(v&0x7000)>>12), 32, pal)
			draw.Draw(img, image.Rect(b%2*32, b/2*32, b%2*32+32, b/2*32+32), micro, image.Point{}, draw.Over)
		}
		return img, nil
	}
	// a megatile's pieces sit top, right, left, bottom on the diamond grid; drawn back to front
	offsets := []image.Point{{32, 0}, {64, 16}, {0, 16}, {32, 32}}
	for m := 0; m < len(til)/8; m++ {
		img := image.NewNRGBA(image.Rect(0, 0, 128, pieceH+32))
		solid := 0
		for i, o := range offsets {
			id := int(le.Uint16(til[m*8+i*2:]))
			p, err := piece(id)
			if err != nil {
				return s, err
			}
			draw.Draw(img, p.Bounds().Add(o), p, image.Point{}, draw.Over)
			if id < len(sol) { // Hellfire's town adds three pieces but keeps Diablo's Town.SOL
				solid += int(sol[id] & 1)
			}
		}
		s.cards = append(s.cards, card{img: crop(img), file: fmt.Sprintf("megatiles/%s/mega%03d.png", name, m+1),
			lines: []string{fmt.Sprintf("mega %d", m+1), fmt.Sprintf("%d/4 pieces solid", solid)}})
	}
	return s, nil
}

// megatileGamma lights megatiles up for reading. Unlit, they already show the brightest light the game has (light
// level 0 leaves the palette as is), which leaves the town murky; the game's own gamma slider brightens with this
// formula down to 0.3, where colors wash out and band, so 0.5 splits the difference.
const megatileGamma = 0.5

// brighten applies ApplyGamma from palette.cpp to a palette.
func brighten(pal []byte) []byte {
	out := make([]byte, len(pal))
	for i, c := range pal {
		out[i] = byte(min(255, 256*math.Pow(float64(c)/256, megatileGamma)))
	}
	return out
}

// decodeMicro expands one 32x32 level CEL frame to bottom-up palette indices. The frame's MIN type says how it is
// stored: raw, CEL RLE, or the left/right half of the floor diamond (a triangle, or a trapezoid with a raw upper
// half) with only the diamond's pixels and two padding bytes every other row, as devilutionX's
// reencode_dun_cels.cpp reads them.
func decodeMicro(f []byte, typ int) []int {
	px := make([]int, 32*32)
	for i := range px {
		px[i] = -1
	}
	switch typ {
	case 0:
		for i := range px {
			px[i] = int(f[i])
		}
		return px
	case 1:
		copy(px, decodeCel(f))
		return px
	}
	left := typ == 2 || typ == 4
	src := 0
	row := func(y, w int, pad bool) {
		if pad && left {
			src += 2
		}
		x := 0
		if left {
			x = 32 - w
		}
		for i := range w {
			px[y*32+x+i] = int(f[src+i])
		}
		src += w
		if pad && !left {
			src += 2
		}
	}
	for y := range 16 { // the lower half widens by two pixels a row from the diamond's bottom tip
		row(y, 2*(y+1), y%2 == 0)
	}
	if typ >= 4 {
		for y := 16; y < 32; y++ {
			row(y, 32, false)
		}
		return px
	}
	for y := 16; y < 31; y++ {
		row(y, 30-2*(y-16), y%2 == 0)
	}
	return px
}
