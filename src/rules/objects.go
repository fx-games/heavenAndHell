package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
)

// objects.cpp: objects placed on level creation.

// ObjData is a row of objdat.cpp's AllObjects that matters after placement.
type ObjData struct {
	Name                                 string
	AnimFlag, AnimDelay, AnimLen         int
	Solid, Missile, Light, Trap          bool
	Break, SelFlag                       int
	MinLvl, MaxLvl, LvlType, Theme, Load int
}

// Object is a placed D1 object (ObjectStruct minus graphics), in piece coordinates.
type Object struct {
	Kind         int // Obj* (row of Data.Objects)
	X, Y         int
	Index        int // D1's object slot; dObject holds Index+1
	AnimFrame    int
	SelFlag      int
	Solid        bool
	Missile      bool
	TrapFlag     bool
	DoorFlag     bool
	PreFlag      bool
	RndSeed      int32
	Var1, Var2   int // doors: pieces; chests: item count, item kind; shrines: type; barrels/sarcophagi: contents
	Var3, Var4   int // traps: missile; barrels/sarcophagi: pre-spawned skeleton slot (-1 none)
	Var5, Var6   int
	Var7, Var8   int      // books: message, lever id
	ExtraCells   [][2]int // cells D1 marks with -(Index+1): sarcophagi, fountains, mushroom patch
	AnimDisabled bool     // _oAnimFlag 2: an empty armor stand or weapon rack
	Active       bool     // game state (not part of D1's placement): true until opened, broken or pulled
}

// shrineCounts are the shrine tables the engine has effects for: D1 1.09's 26 (NUM_SHRINETYPE) and Hellfire's 34,
// which appends eight. The generator rolls an index into the mod's table, so any other length would quietly change
// which shrines appear.
var shrineCounts = []int{26, 34}

// ObjectDisplay is what the game shows and plays for an object kind: the parts of it that are presentation
// rather than placement, which D1 spread over a handful of small tables.
type ObjectDisplay struct {
	Name      string // display name; "" for decoration the game never names
	Light     int    // light radius of the ones that glow (ProcessObjects' Obj_Light), 0 for the rest
	Container bool   // looted by opening or breaking, and may give nothing
	WallLight bool   // a decorative light D1 places directly
	Sfx       string // the sound it makes as it is used
}

func loadObjectTables(fsys fs.FS, d *Data) error {
	recs, err := readRecMulti(fsys, "objects.rec")
	// A mod that does not have this Diablo feature ships no file for it, and an absent feature is not an
	// error: what the table would have filled stays empty.
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	kindByName := make(map[string]int, len(d.Objects))
	for i, o := range d.Objects {
		kindByName[o.Name] = i
	}
	d.ObjectDisplays = make(map[int]ObjectDisplay, len(recs["object"]))
	for _, r := range recs["object"] {
		m := r.ToMap("")
		id := m.GetStringOrDefault("id", "")
		kind, ok := kindByName[id]
		if !ok {
			return fmt.Errorf("objects.rec: unknown object %q", id)
		}
		d.ObjectDisplays[kind] = ObjectDisplay{
			Name:      m.GetStringOrDefault("name", ""),
			Light:     m.GetIntOrDefault("light", 0),
			Container: m.GetBoolOrFalse("container"),
			WallLight: m.GetBoolOrFalse("wall_light"),
			Sfx:       m.GetStringOrDefault("sfx", ""),
		}
	}
	d.ShrineNames, d.ShrineMessages = nil, nil
	d.ShrineMax, d.ShrineAvail = nil, nil
	for _, r := range recs["shrine"] {
		m := r.ToMap("")
		d.ShrineNames = append(d.ShrineNames, m.GetStringOrDefault("name", ""))
		d.ShrineMessages = append(d.ShrineMessages, m.GetStringOrDefault("message", ""))
		d.ShrineMax = append(d.ShrineMax, m.GetIntOrDefault("max_lvl", 0))
		d.ShrineAvail = append(d.ShrineAvail, m.GetIntOrDefault("avail", 0))
	}
	if !slices.Contains(shrineCounts, len(d.ShrineNames)) {
		return fmt.Errorf("objects.rec: %d shrines, want one of %v", len(d.ShrineNames), shrineCounts)
	}
	d.StoryBookNames = nil
	for _, r := range recs["storybook"] {
		d.StoryBookNames = append(d.StoryBookNames, r.ToMap("").GetStringOrDefault("name", ""))
	}
	return nil
}
