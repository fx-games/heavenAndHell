package defs

import (
	"fmt"
	"path"
	"regexp"
	"rpg/files"
	"rpg/src/recfile"
	"strings"
)

// Icons live in four independently remappable rec files under files/mods/<mod>/data/icons/ (8x8 tiles in
// the sprite sheet named by common.Configuration.SpriteSheet), one ID convention throughout: "kind.snake_case_name", e.g. "class.warrior_male",
// "monster.nzombie", "unique.gharbad_the_weak", "npc.wirt", "corpse.default" (actors.rec); "item.falchion",
// "item.falchion.magic"/".unique" (per weapon/armor family), "item.heal" (potions/scrolls, keyed by IMISC_*) etc.
// (items.rec); "tile.grass", "prop.chest1"(+".used"), "decal.blood1", "effect.fire_small" (tiles.rec);
// "spell.firebolt" (skills.rec). IDs built from D1's own enum names (MT_*, OBJ_*, SPL_*, ITYPE_*, IMISC_*) go
// through IconID, which strips the enum prefix and lowercases the rest. Remap them with scripts/remap_actors.sh,
// scripts/remap_items.sh, scripts/remap_tiles.sh, scripts/remap_skills.sh — each rec file is read fresh on every game start, no
// rebuild needed. The H&H constants (IconGrass, ...) are package vars, not consts, for the same
// reason: their value comes from the rec file, not from the Go source.
//
// The sprite sheet they index is the active mod's: a mod that ships its own assets/data/icons and its own sheet
// gets both. That is why LoadIcons is explicit rather than a package initializer -- the vars must be filled
// AFTER files.SelectMod, or every Icon* silently carries the engine's number instead of the mod's.

// missingIcons collects every name a mod's icons/*.rec did not define, so LoadIcons can name all of them at
// once instead of the load dying on the first and the rest silently rendering tile 0.
var missingIcons []string

// namedIcon is the lookup behind every Icon* package var. A name the mod did not supply is recorded rather
// than resolving to 0: content binding silently to nothing has caused four bugs here, and a second mod with
// its own icon names is exactly when an unlisted name stops being a typo and starts being a whole game.
func namedIcon(m map[string]int32, name string) int32 {
	icon, ok := m[name]
	if !ok {
		missingIcons = append(missingIcons, name)
	}
	return icon
}

// LoadIcons fills every Icon* var from the active mod's icons/*.rec. It fails if the mod is missing any of
// them, listing the lot, because a half-bound icon table renders tile 0 and says nothing.
func LoadIcons() error {
	missingIcons = nil
	loadActorsIcons()
	loadItemsIcons()
	loadSkillsIcons()
	loadTilesIcons()
	if len(missingIcons) > 0 {
		return fmt.Errorf("mod %q defines no icon named: %s", files.ActiveMod(), strings.Join(missingIcons, ", "))
	}
	return nil
}

func loadIconRec(name string) map[string]int32 {
	f, err := files.Open(path.Join("assets", "data", "icons", name+".rec"))
	if err != nil {
		panic(err)
	}
	defer f.Close()
	icons, sameAs := map[string]int32{}, map[string]string{}
	for _, rec := range recfile.Read(f) {
		var n, alias string
		var icon int32
		hasIcon := false
		for _, field := range rec {
			switch field.Name {
			case "internal_name":
				n = field.Value
			case "icon":
				icon, hasIcon = field.AsInt32(), true
			case "same_as":
				alias = field.Value
			}
		}
		if alias != "" && !hasIcon {
			sameAs[n] = alias
			continue
		}
		icons[n] = icon
	}
	// same_as: a record drawn with another's icon (a trapped chest looks like a chest), unless it has its own.
	for n, target := range sameAs {
		if icon, ok := icons[target]; ok {
			icons[n] = icon
		} else {
			missingIcons = append(missingIcons, n+" (same_as "+target+")")
		}
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

// PropIconID is the icon id of a dungeon object: IconID("prop", obj). Objects the top-down view can't tell apart
// share an icon through tiles.rec's same_as.
func PropIconID(obj string) string { return IconID("prop", obj) }

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
