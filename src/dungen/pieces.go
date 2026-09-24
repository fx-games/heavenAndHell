package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"
	"path"
	"strings"

	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/hhm"
	"rpg/src/recfile"
)

// PieceSpec is one of a level's set pieces (levels.rec's "set_piece: <map> <placement> [quest]"): a map from the
// mod's maps (or one of several, "a|b|c", picked per level) laid into the level, only while the quest is available on it when one is named. Placements:
//
//	chamber   one of the spine's chambers (D1's cathedral quest rooms, l1SelectChamber)
//	room      a room of its own, where it breaks the least floor; corridors join it (D1's reserved rooms)
//	open      where the most open ground takes it in whole (the Anvil's island, the stairs minisets)
//	quad0-3   D1's level 16: its four quadrants, mirrored round the level's centre (l4LoadDiabQuads)
type PieceSpec struct{ Map, Place, Quest string }

// Stamped is a set piece as laid into a level: its map, the layout it opens into (nil for none: levels.rec's
// patches, D1's ObjChangeMap), and its top-left corner in cells.
type Stamped struct {
	Map, Open *hhm.Level
	X, Y      int
}

func parsePieces(r recfile.Record) ([]PieceSpec, error) {
	var out []PieceSpec
	for _, v := range dungeon.FieldsNamed(r, "set_piece") {
		f := strings.Fields(v)
		if len(f) < 2 || len(f) > 3 {
			return nil, fmt.Errorf("levels.rec: set_piece %q: want <map> <placement> [quest]", v)
		}
		s := PieceSpec{Map: f[0], Place: f[1]}
		if len(f) == 3 {
			s.Quest = f[2]
		}
		switch s.Place {
		case "chamber", "room", "open", "quad0", "quad1", "quad2", "quad3":
		default:
			return nil, fmt.Errorf("levels.rec: set_piece %q: unknown placement %q", v, s.Place)
		}
		out = append(out, s)
	}
	return out, nil
}

// WithQuests tells a level's configuration which quests are available on it, so the set pieces of the others stay
// out. A configuration without set pieces comes back as it was.
func WithQuests(cfg dungeon.Config, avail func(quest string) bool) dungeon.Config {
	if s, ok := cfg.(staged); ok {
		s.avail = avail
		return s
	}
	return cfg
}

// placed is a piece chosen for the level and where its footprint went on the grid.
type placed struct {
	Stamped
	mx, my, mw, mh int // footprint in grid units
}

// placePieces lays the level's pieces' footprints onto the grid (as mPiece, which the passes after it leave
// alone and connect joins up). chambers are the spine's, in the grid's coordinates.
func (s staged) placePieces(m *mega, fsys fs.FS, chambers [][2]int, rnd *rand.Rand) ([]placed, error) {
	var specs []PieceSpec
	for _, spec := range s.Pieces {
		if spec.Quest == "" || s.avail != nil && s.avail(spec.Quest) {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		return nil, nil
	}
	proj, err := loadTemplates(fsys, hhm.Dir)
	if err != nil {
		return nil, err
	}
	sc := s.Scale
	var out []placed
	hold := 3 + rnd.Intn(3) // D1's level 16 first room corner (l4FirstRoom: 14x14 somewhere in 3..5)
	for _, spec := range specs {
		if alts := strings.Split(spec.Map, "|"); len(alts) > 1 { // variants: one per level
			spec.Map = alts[rnd.Intn(len(alts))]
		}
		lv := proj.Level(spec.Map)
		if lv == nil {
			return nil, fmt.Errorf("set_piece: the maps have no %s", spec.Map)
		}
		p := placed{Stamped: Stamped{Map: lv, Open: proj.Level(spec.Map + "_open")}, mw: (lv.Width + sc - 1) / sc, mh: (lv.Height + sc - 1) / sc}
		free := func(x, y int) bool {
			if x < 1 || y < 1 || x+p.mw >= m.w || y+p.mh >= m.h {
				return false
			}
			for _, o := range out {
				if x < o.mx+o.mw+1 && o.mx < x+p.mw+1 && y < o.my+o.mh+1 && o.my < y+p.mh+1 {
					return false
				}
			}
			return true
		}
		ok := false
		switch spec.Place {
		case "chamber":
			for _, i := range rnd.Perm(len(chambers)) {
				if x, y := chambers[i][0]+2, chambers[i][1]+2; free(x, y) {
					p.mx, p.my, ok = x, y, true
					break
				}
			}
		case "quad0", "quad1", "quad2", "quad3":
			q := [4][2]int{{hold + 4, hold + 4}, {27 - hold, 1 + hold}, {1 + hold, 27 - hold}, {28 - hold, 28 - hold}}[spec.Place[4]-'0']
			p.mx, p.my, ok = q[0]+(m.w-40)/2, q[1]+(m.h-40)/2, true
		default: // room, open: the best of many tries
			best := -1
			for try := 0; try < 400; try++ {
				x, y := 1+rnd.Intn(max(m.w-p.mw-1, 1)), 1+rnd.Intn(max(m.h-p.mh-1, 1))
				if !free(x, y) {
					continue
				}
				n := 0
				for j := y - 1; j <= y+p.mh; j++ {
					for i := x - 1; i <= x+p.mw; i++ {
						if mopen(m.at(i, j)) == (spec.Place == "open") {
							n++
						}
					}
				}
				if n > best {
					best, p.mx, p.my, ok = n, x, y, true
				}
			}
		}
		if !ok {
			return nil, fmt.Errorf("set_piece %s: no room for its %s placement", spec.Map, spec.Place)
		}
		for j := p.my; j < p.my+p.mh; j++ {
			for i := p.mx; i < p.mx+p.mw; i++ {
				m.set(i, j, mPiece)
			}
		}
		out = append(out, p)
	}
	return out, nil
}

// stampPieces draws the pieces into the finished map cell by cell, from their tiles: what a tile blocks decides
// the cell (a door where the map has one), undrawn cells keep the layout's. Then connectCells joins every piece's
// open ground to the level, never through a piece: rooms a piece seals (opened later) stay sealed.
func stampPieces(dm *DungeonMap, ps []placed, fsys fs.FS, sc int) error {
	if len(ps) == 0 {
		return nil
	}
	proj, err := loadTemplates(fsys, hhm.Dir)
	if err != nil {
		return err
	}
	recs, err := hhm.ReadTileRecords(fsys, path.Join("assets", "data"))
	if err != nil {
		return err
	}
	locked := make([]bool, len(dm.tiles))
	for _, p := range ps {
		p.X, p.Y = p.mx*sc, p.my*sc
		lv := p.Map
		doors := map[[2]int]bool{}
		for _, e := range lv.Entities {
			if e.Kind == "Door" {
				doors[[2]int{e.X, e.Y}] = true
			}
		}
		inside := make([]bool, lv.Width*lv.Height)
		for y := 0; y < lv.Height; y++ {
			for x := 0; x < lv.Width; x++ {
				id := lv.TileAt(x, y)
				if id == hhm.Empty || !dm.Contains(pt(p.X+x, p.Y+y)) {
					continue
				}
				i := p.X + x + (p.Y+y)*dm.width
				dm.tiles[i] = cellKind(hhm.DrawnTile(proj.Tileset, recs, id).Flags, doors[[2]int{x, y}])
				locked[i], inside[x+y*lv.Width] = true, true
			}
		}
		// the piece's ground its own edge reaches is the level's to join; what it walls off stays locked
		var q [][2]int
		seen := make([]bool, len(inside))
		for y := 0; y < lv.Height; y++ {
			for x := 0; x < lv.Width; x++ {
				if (x == 0 || y == 0 || x == lv.Width-1 || y == lv.Height-1) && inside[x+y*lv.Width] && dm.IsWalkable(pt(p.X+x, p.Y+y)) {
					seen[x+y*lv.Width] = true
					q = append(q, [2]int{x, y})
				}
			}
		}
		for ; len(q) > 0; q = q[1:] {
			c := q[0]
			locked[p.X+c[0]+(p.Y+c[1])*dm.width] = false
			for _, d := range mdirs {
				x, y := c[0]+d[0], c[1]+d[1]
				if x >= 0 && y >= 0 && x < lv.Width && y < lv.Height && !seen[x+y*lv.Width] && inside[x+y*lv.Width] && dm.IsWalkable(pt(p.X+x, p.Y+y)) {
					seen[x+y*lv.Width] = true
					q = append(q, [2]int{x, y})
				}
			}
		}
		dm.pieces = append(dm.pieces, p.Stamped)
	}
	connectCells(dm, locked)
	return nil
}

// cellKind is the layout tile a drawn tile makes.
func cellKind(f hhm.Flags, door bool) DungeonTile {
	switch {
	case door:
		return Door
	case f.Damaging && f.BlocksMovement:
		return Lava
	case f.Water && f.BlocksMovement:
		return Lake
	case f.BlocksMovement && f.BlocksView:
		return Wall
	case f.BlocksMovement:
		return Pillar
	}
	return Room
}

// connectCells runs connectMega at cell scale: every walkable cell not locked ends up joined, and nothing
// locked is carved or counted.
func connectCells(dm *DungeonMap, locked []bool) {
	m := newMega(dm.width, dm.height)
	for i, t := range dm.tiles {
		switch {
		case locked[i]:
			m.t[i] = mLocked
		case t == Room || t == Corridor || t == Shallows:
			m.t[i] = mRoom
		case t == Door:
			m.t[i] = mDoor
		case t == Lava || t == Lake:
			m.t[i] = mLava
		}
	}
	connectMega(m)
	for i, k := range m.t {
		if t := dm.tiles[i]; !locked[i] && (k == mCorr || k == mRoom) && (t == Wall || t == Pillar || t == Lava || t == Lake) {
			dm.tiles[i] = Corridor
		}
	}
}

func pt(x, y int) geometry.Point { return geometry.Point{X: x, Y: y} }
