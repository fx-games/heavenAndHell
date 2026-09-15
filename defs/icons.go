package defs

import (
	"path"
	"rpg/files"
	"rpg/recfile"
)

// Icons live in three independently remappable rec files under files/assets/data/icons/ (8x8 tiles in
// assets/classic_roguelike.png): actors.rec (H&H IconCharacter*/IconMonster*/IconUnique*/IconCorpse*/IconRogue*
// plus D1's "monster.<ID>", "unique.<Name>" and "npc.<name>"), items.rec (IconItem* plus D1's "item.ITYPE_*"/"item.IMISC_*"),
// tiles.rec (everything else: terrain, doors, decals, effects, plus D1's "prop.OBJ_*"). Remap them with
// ./remap_actors.sh, ./remap_items.sh, ./remap_tiles.sh — each rec file is read fresh on every game start, no
// rebuild needed. The H&H constants (IconGrass, IconItemSword, ...) are package vars, not consts, for the same
// reason: their value comes from the rec file, not from the Go source.
func loadIconRec(name string) map[string]int32 {
	f, err := files.Open(path.Join("assets", "data", "icons", name+".rec"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	icons := map[string]int32{}
	for _, rec := range recfile.Read(f) {
		var n string
		var icon int32
		for _, field := range rec {
			switch field.Name {
			case "internal_name":
				n = field.Value
			case "icon":
				icon = field.AsInt32()
			}
		}
		icons[n] = icon
	}
	return icons
}

// IconOK looks up a name not exposed as a Go constant (D1's dynamically keyed "monster.<ID>", "npc.<name>",
// "item.ITYPE_*"/"item.IMISC_*", "prop.OBJ_*") in all three category files.
func IconOK(name string) (int32, bool) {
	for _, m := range []map[string]int32{actorsIcons, itemsIcons, tilesIcons} {
		if icon, ok := m[name]; ok {
			return icon, true
		}
	}
	return 0, false
}

// Icon is IconOK but panics on a typo instead of silently drawing the wrong tile.
func Icon(name string) int32 {
	icon, ok := IconOK(name)
	if !ok {
		panic("no icon named " + name)
	}
	return icon
}
