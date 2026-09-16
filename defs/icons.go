package defs

import (
	"path"
	"regexp"
	"rpg/files"
	"rpg/recfile"
	"strings"
)

// Icons live in four independently remappable rec files under files/assets/data/icons/ (8x8 tiles in
// assets/classic_roguelike.png), one ID convention throughout: "kind.snake_case_name", e.g. "class.warrior_male",
// "monster.nzombie", "unique.gharbad_the_weak", "npc.wirt", "corpse.default" (actors.rec); "item.falchion",
// "item.falchion.magic"/".unique" (per weapon/armor family), "item.heal" (potions/scrolls, keyed by IMISC_*) etc.
// (items.rec); "tile.grass", "prop.chest1"(+".used"), "decal.blood1", "effect.fire_small" (tiles.rec);
// "spell.firebolt" (skills.rec). IDs built from D1's own enum names (MT_*, OBJ_*, SPL_*, ITYPE_*, IMISC_*) go
// through IconID, which strips the enum prefix and lowercases the rest. Remap them with ./remap_actors.sh,
// ./remap_items.sh, ./remap_tiles.sh, ./remap_skills.sh — each rec file is read fresh on every game start, no
// rebuild needed. The H&H constants (IconGrass, ...) are package vars, not consts, for the same
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

var iconIDPrefixes = []string{"MT_", "OBJ_", "SPL_", "ITYPE_", "IMISC_"}
var iconIDNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// IconID builds a "kind.snake_case_name" icon ID from a D1 enum value or free-text name, stripping the enum's
// own prefix (MT_NZOMBIE -> nzombie, OBJ_CHEST1 -> chest1, "Gharbad the Weak" -> gharbad_the_weak) so every
// rec file uses one convention regardless of where the raw name came from.
func IconID(kind, raw string) string {
	for _, p := range iconIDPrefixes {
		if trimmed, ok := strings.CutPrefix(raw, p); ok {
			raw = trimmed
			break
		}
	}
	slug := strings.Trim(iconIDNonWord.ReplaceAllString(strings.ToLower(raw), "_"), "_")
	return kind + "." + slug
}

// IconOK looks up a name not exposed as a Go constant (D1's dynamically keyed "monster.<id>", "npc.<name>",
// "item.<name>", "prop.<name>", "spell.<name>", built with IconID) in all four category files.
func IconOK(name string) (int32, bool) {
	for _, m := range []map[string]int32{actorsIcons, itemsIcons, tilesIcons, skillsIcons} {
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
