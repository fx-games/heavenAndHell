package dungen

import (
	"fmt"
	"io/fs"
	"math/rand"

	"rpg/dungeon"
	"rpg/geometry"
	"rpg/ldtk"
	"rpg/recfile"
)

// This is H&H's side of the engine's generation registry (package dungeon). The algorithms below are genuinely
// different from D1's - they grow a level out of hand-drawn room templates, or carve a maze and knock holes in
// it - so none of D1's tile sets, megatile fills or quest set pieces appear here, and none of this appears in
// the shared interface. Both families meet only at "a name resolves, and the knobs a mod declared parse".

func init() {
	dungeon.Register("accretion", accretion{})
	dungeon.Register("megadungeon", megadungeon{})
}

// scratchSize is the grid an algorithm grows into before it is trimmed. It is a knob, but these are the sizes
// H&H shipped with.
const (
	accretionScratch   = 40
	megadungeonScratch = 60
)

// AccretionConfig is what a mod declares for the accretion generator: which hand-drawn room templates it may
// draw from, which it must place, and how many rooms make a level.
type AccretionConfig struct {
	Templates     string   // the LDtk project the room templates live in, e.g. assets/rooms.ldtk
	Prefix        string   // only templates whose name starts with this are used - H&H's tile set per depth
	Require       []string // templates this level must contain (its quest rooms)
	Forbid        []string // templates this level must not contain (another level's quest rooms)
	Rooms         int      // how many rooms to grow to
	MinRooms      int      // regrow the level until it has at least this many
	Width, Height int
	Doors         bool
}

type accretion struct{}

func (accretion) Parse(r recfile.Record) (dungeon.Config, error) {
	c := AccretionConfig{Templates: "assets/rooms.ldtk", Rooms: 16, MinRooms: 10,
		Width: accretionScratch, Height: accretionScratch, Doors: true}
	if v, ok := dungeon.Field(r, "templates"); ok {
		c.Templates = v
	}
	c.Prefix, _ = dungeon.Field(r, "template_prefix")
	if c.Prefix == "" {
		return nil, fmt.Errorf("levels.rec: accretion needs a template_prefix")
	}
	c.Require = dungeon.FieldsNamed(r, "require_room")
	c.Forbid = dungeon.FieldsNamed(r, "forbid_room")
	if v, ok := dungeon.Field(r, "doors"); ok {
		c.Doors = v != "false"
	}
	var err error
	for _, k := range []struct {
		name string
		into *int
	}{{"rooms", &c.Rooms}, {"min_rooms", &c.MinRooms}, {"width", &c.Width}, {"height", &c.Height}} {
		if *k.into, err = dungeon.Int(r, k.name, *k.into); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// MegaDungeonConfig is what a mod declares for the maze-and-rooms generator. The knobs were rolled off the
// package-level RNG when this generator was written, which made a level unreproducible from its seed; they are
// parameters now, with the midpoints of the old rolls as defaults.
type MegaDungeonConfig struct {
	Width, Height          int
	RoomTries              int
	MinRoomSize            int
	StraightPassageChance  float64
	ImperfectConnectChance float64
	RoomRatioInterval      float64
	DoorDistance           int
	RemoveDeadEnds         bool
	MinRooms               int
}

type megadungeon struct{}

func (megadungeon) Parse(r recfile.Record) (dungeon.Config, error) {
	c := MegaDungeonConfig{Width: megadungeonScratch, Height: megadungeonScratch, RoomTries: 1266, MinRoomSize: 5,
		StraightPassageChance: 0.5, ImperfectConnectChance: 0.5, RoomRatioInterval: 0.25, DoorDistance: 5,
		RemoveDeadEnds: true, MinRooms: 10}
	var err error
	for _, k := range []struct {
		name string
		into *int
	}{{"width", &c.Width}, {"height", &c.Height}, {"room_tries", &c.RoomTries},
		{"min_room_size", &c.MinRoomSize}, {"door_distance", &c.DoorDistance}, {"min_rooms", &c.MinRooms}} {
		if *k.into, err = dungeon.Int(r, k.name, *k.into); err != nil {
			return nil, err
		}
	}
	for _, k := range []struct {
		name string
		into *float64
	}{{"straight_passage_chance", &c.StraightPassageChance},
		{"imperfect_connect_chance", &c.ImperfectConnectChance}, {"room_ratio", &c.RoomRatioInterval}} {
		if v, ok := dungeon.Field(r, k.name); ok {
			if _, err := fmt.Sscanf(v, "%f", k.into); err != nil {
				return nil, fmt.Errorf("levels.rec: %s %q: %w", k.name, v, err)
			}
		}
	}
	if v, ok := dungeon.Field(r, "dead_ends"); ok {
		c.RemoveDeadEnds = v != "true" // "dead_ends: true" keeps them; they are filled in otherwise
	}
	return c, nil
}

// Build runs whichever of these algorithms a level's configuration names and returns its layout. It is dungen's
// own entry point, not part of dungeon.Strategy: D1 hands its caller a populated d1.Level and this hands back a
// bare DungeonMap, and pretending those are the same return type is what the narrow interface avoids.
//
// fsys is the active mod's file system, for the generators that read room templates from it.
func Build(cfg dungeon.Config, rnd *rand.Rand, fsys fs.FS) (*DungeonMap, error) {
	switch c := cfg.(type) {
	case AccretionConfig:
		return buildAccretion(c, rnd, fsys)
	case MegaDungeonConfig:
		return buildMegaDungeon(c, rnd), nil
	}
	return nil, fmt.Errorf("dungen: %T is not one of this family's configurations", cfg)
}

func buildAccretion(c AccretionConfig, rnd *rand.Rand, fsys fs.FS) (*DungeonMap, error) {
	proj, err := loadTemplates(fsys, c.Templates)
	if err != nil {
		return nil, err
	}
	rules := TemplateRules{RequireTemplates: map[string]bool{}, ForbidTemplates: map[string]bool{},
		TemplatePrefix: c.Prefix, NoDoors: !c.Doors}
	for _, n := range c.Require {
		rules.RequireTemplates[n] = true
	}
	for _, n := range c.Forbid {
		rules.ForbidTemplates[n] = true
	}
	g := NewAccretionGenerator(rnd)
	g.LoadTemplatesFrom(proj, c.Prefix)
	g.SetLevelRules(rules)
	g.SetRoomCount(c.Rooms)
	if len(g.roomTemplates) == 0 {
		return nil, fmt.Errorf("dungen: %s has no room templates with prefix %q", c.Templates, c.Prefix)
	}
	// The generator grows a level and can grow a stunted one; H&H regrew it until it was big enough. Bounded,
	// because "keep rolling until it is good" with no bound is a hang waiting for a bad seed.
	var why string
	for try := 0; try < 32; try++ {
		m, err := g.Generate(c.Width, c.Height)
		if err != nil {
			return nil, err
		}
		if missing := g.MissingRequired(); missing != "" {
			why = "never placed required room " + missing
		} else if n := len(m.AllRooms()); n < c.MinRooms {
			why = fmt.Sprintf("grew at most %d of min_rooms %d", n, c.MinRooms)
		} else {
			return m, nil
		}
	}
	return nil, fmt.Errorf("dungen: %s level %s in 32 tries", c.Prefix, why)
}

func buildMegaDungeon(c MegaDungeonConfig, rnd *rand.Rand) *DungeonMap {
	var m *DungeonMap
	for try := 0; try < 32; try++ {
		g := &MegaDungeonGenerator{randomSource: rnd, roomTries: c.RoomTries, minRoomSize: c.MinRoomSize,
			straightPassageChance: c.StraightPassageChance, imperfectConnectChance: c.ImperfectConnectChance,
			roomRatioInterval: c.RoomRatioInterval, doorDistance: c.DoorDistance,
			removeDeadEnds: c.RemoveDeadEnds, regionLookup: map[geometry.Point]Region{}}
		m = g.Generate(c.Width, c.Height)
		if len(m.AllRooms()) >= c.MinRooms {
			return m
		}
	}
	return m
}

// Templates returns the room-template project a configuration draws from, or nil for a generator that draws
// from none (megadungeon carves its rooms rather than stamping them). The caller wants it to paint the
// hand-drawn art of a room over the floor the tile set painted, which is the only thing outside this package
// that needs to see the templates at all.
func Templates(cfg dungeon.Config, fsys fs.FS) (*ldtk.Project, error) {
	c, ok := cfg.(AccretionConfig)
	if !ok {
		return nil, nil
	}
	return loadTemplates(fsys, c.Templates)
}

// loadTemplates reads an LDtk project out of the active mod.
func loadTemplates(fsys fs.FS, name string) (*ldtk.Project, error) {
	b, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, fmt.Errorf("dungen templates %s: %w", name, err)
	}
	p, err := ldtk.Read(b)
	if err != nil {
		return nil, fmt.Errorf("dungen templates %s: %w", name, err)
	}
	return p, nil
}
