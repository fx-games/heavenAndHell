// Package hhm is Heaven & Hell's map format: one plain-text file per level, plus one tileset file per mod.
// It replaces the LDtk project file the game used to read; nothing in the engine knows LDtk any more.
//
// A mod's maps live in <mod>/maps: tileset.hht and one <Name>.hhm per level.
package hhm

import (
	"sort"
	"strconv"
	"strings"
)

// Dir is the maps directory inside a mod, as the game's file system sees it.
const Dir = "assets/maps"

// TilesetFile is the tileset definition inside Dir; Ext is the extension of one level file.
const (
	TilesetFile = "tileset.hht"
	Ext         = ".hhm"
)

// Flags are the per-tile markers the tileset records; they become the tile's gridmap properties.
type Flags struct {
	BlocksMovement bool
	BlocksView     bool
	Water          bool
	Damaging       bool
	BlocksMissiles bool
	HoldsTrap      bool // a wall a trap may be set into, facing the open cell beside it (D1's traps and shrines)
}

// FlagNames are the markers in the order a tileset file writes them.
var FlagNames = []string{"IsBlockingMovement", "IsBlockingView", "IsWater", "IsDamaging", "IsBlockingMissiles", "CanHoldTrap"}

// Names lists the markers that are on, in the order a tileset file writes them.
func (f Flags) Names() []string {
	var out []string
	for i, on := range [...]bool{f.BlocksMovement, f.BlocksView, f.Water, f.Damaging, f.BlocksMissiles, f.HoldsTrap} {
		if on {
			out = append(out, FlagNames[i])
		}
	}
	return out
}

func (f *Flags) set(name string) {
	switch strings.TrimSpace(name) {
	case FlagNames[0]:
		f.BlocksMovement = true
	case FlagNames[1]:
		f.BlocksView = true
	case FlagNames[2]:
		f.Water = true
	case FlagNames[3]:
		f.Damaging = true
	case FlagNames[4]:
		f.BlocksMissiles = true
	case FlagNames[5]:
		f.HoldsTrap = true
	}
}

func (f Flags) any() bool { return f != Flags{} }

// Tileset is the one image every level draws from, and what its tiles mean.
type Tileset struct {
	Name     string
	Image    string // relative to the mod's root, like the rest of a mod's assets
	GridSize int
	Width    int // in pixels
	Height   int
	Flags    map[int]Flags // by tile index; a tile with no entry is plain walkable floor
	// Records names the tiles.rec record a drawn tile stands for ("183" -> tile.cathedral_trap_wall): the game
	// takes its name, icon and what it blocks from the record, not from Flags, so remapping the record reaches
	// every map that draws it.
	Records map[int]string
}

// Columns is how many tiles fit in one row of the image.
func (t *Tileset) Columns() int {
	if t.GridSize == 0 {
		return 0
	}
	return t.Width / t.GridSize
}

// Count is how many tiles the image holds.
func (t *Tileset) Count() int {
	if t.GridSize == 0 {
		return 0
	}
	return t.Columns() * (t.Height / t.GridSize)
}

// FlagsFor returns the markers on a tile.
func (t *Tileset) FlagsFor(id int) Flags { return t.Flags[id] }

// Props are the fields a level carries, all of which some generator reads.
type Props struct {
	DisplayName     string
	Weight          int
	IsDecoration    bool
	UsesDecoration  string // the name prefix of the decoration levels this template wants, "" for none
	SlotID          int
	PlugsIntoSlotID int
	CanBeLit        bool
}

// Level is one map or room template.
type Level struct {
	Name     string
	Width    int // in cells
	Height   int
	Tiles    []int // Width*Height tile indices, row by row; Empty where nothing is drawn
	Entities []*Entity
	Props    Props
}

// Empty is what an undrawn cell holds.
const Empty = -1

// NewLevel makes an empty level of that size.
func NewLevel(name string, w, h int) *Level {
	l := &Level{Name: name, Width: w, Height: h, Tiles: make([]int, w*h), Props: Props{DisplayName: name}}
	for i := range l.Tiles {
		l.Tiles[i] = Empty
	}
	return l
}

// Contains reports whether the cell is inside the level.
func (l *Level) Contains(x, y int) bool { return x >= 0 && y >= 0 && x < l.Width && y < l.Height }

// TileAt returns the tile drawn at the cell, or Empty.
func (l *Level) TileAt(x, y int) int {
	if !l.Contains(x, y) {
		return Empty
	}
	return l.Tiles[y*l.Width+x]
}

// SetTile draws a tile at the cell.
func (l *Level) SetTile(x, y, id int) {
	if l.Contains(x, y) {
		l.Tiles[y*l.Width+x] = id
	}
}

// Resize grows or crops the level, keeping what still fits.
func (l *Level) Resize(w, h int) {
	if w <= 0 || h <= 0 {
		return
	}
	tiles := make([]int, w*h)
	for i := range tiles {
		tiles[i] = Empty
	}
	for y := 0; y < min(h, l.Height); y++ {
		copy(tiles[y*w:y*w+min(w, l.Width)], l.Tiles[y*l.Width:])
	}
	kept := l.Entities[:0]
	for _, e := range l.Entities {
		if e.X < w && e.Y < h {
			kept = append(kept, e)
		}
	}
	l.Entities, l.Tiles, l.Width, l.Height = kept, tiles, w, h
}

// EntitiesAt returns every entity standing on that cell.
func (l *Level) EntitiesAt(x, y int) []*Entity {
	var out []*Entity
	for _, e := range l.Entities {
		if e.X == x && e.Y == y {
			out = append(out, e)
		}
	}
	return out
}

// Remove deletes an entity from the level.
func (l *Level) Remove(target *Entity) {
	for i, e := range l.Entities {
		if e == target {
			l.Entities = append(l.Entities[:i], l.Entities[i+1:]...)
			return
		}
	}
}

// Entity is one placed thing: its kind names which fields it carries, see EntityKinds.
type Entity struct {
	Kind string
	X, Y int
	F    map[string]string // written values, defaults filled in on load
}

// NewEntity makes an entity of that kind with every field at its default.
func NewEntity(kind string, x, y int) *Entity {
	e := &Entity{Kind: kind, X: x, Y: y, F: map[string]string{}}
	if k, ok := KindByName(kind); ok {
		for _, f := range k.Fields {
			e.F[f.Name] = f.Default
		}
	}
	return e
}

// Clone copies the entity, fields and all.
func (e *Entity) Clone() *Entity {
	c := &Entity{Kind: e.Kind, X: e.X, Y: e.Y, F: make(map[string]string, len(e.F))}
	for k, v := range e.F {
		c.F[k] = v
	}
	return c
}

// Str, Int and Bool read a field; a field that was never written reads as the zero value.
func (e *Entity) Str(name string) string { return e.F[name] }

func (e *Entity) Int(name string) int { v, _ := strconv.Atoi(strings.TrimSpace(e.F[name])); return v }

func (e *Entity) Bool(name string) bool { return e.F[name] == "true" }

// Project is a mod's whole set of maps.
type Project struct {
	Tileset *Tileset
	Levels  []*Level
}

// Level returns the level with that name, or nil.
func (p *Project) Level(name string) *Level {
	for _, l := range p.Levels {
		if l.Name == name {
			return l
		}
	}
	return nil
}

// LevelsWithPrefix returns every level whose name starts with the prefix, in file order.
func (p *Project) LevelsWithPrefix(prefix string) []*Level {
	var out []*Level
	for _, l := range p.Levels {
		if strings.HasPrefix(l.Name, prefix) {
			out = append(out, l)
		}
	}
	return out
}

// Add puts a level into the project, keeping the levels sorted by name as the files are.
func (p *Project) Add(l *Level) {
	p.Levels = append(p.Levels, l)
	sort.Slice(p.Levels, func(i, j int) bool { return p.Levels[i].Name < p.Levels[j].Name })
}
