package defs

import (
	"fmt"
	"path"
	"regexp"
	"rpg/files"
	"rpg/recfile"
	"strings"
)

// Icons live in four independently remappable rec files under files/mods/<mod>/data/icons/ (8x8 tiles in
// the sprite sheet named by common.Configuration.SpriteSheet), one ID convention throughout: "kind.snake_case_name", e.g. "class.warrior_male",
// "monster.nzombie", "unique.gharbad_the_weak", "npc.wirt", "corpse.default" (actors.rec); "item.falchion",
// "item.falchion.magic"/".unique" (per weapon/armor family), "item.heal" (potions/scrolls, keyed by IMISC_*) etc.
// (items.rec); "tile.grass", "prop.chest1"(+".used"), "decal.blood1", "effect.fire_small" (tiles.rec);
// "spell.firebolt" (skills.rec). IDs built from D1's own enum names (MT_*, OBJ_*, SPL_*, ITYPE_*, IMISC_*) go
// through IconID, which strips the enum prefix and lowercases the rest. Remap them with ./remap_actors.sh,
// ./remap_items.sh, ./remap_tiles.sh, ./remap_skills.sh — each rec file is read fresh on every game start, no
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

// propFolds folds D1 objects into one icon where the top-down view can't tell them apart: the same sprite under
// another ID (trapped chests, quest copies), or the same thing drawn from another isometric angle (left/right
// walls). A fold to a ".used" key is an object that looks like another's used state (an empty bookcase).
var propFolds = map[string]string{
	"tchest1": "chest1", "tchest2": "chest2", "tchest3": "chest3", "signchest": "chest3",
	"shrinel": "shrine", "shriner": "shrine",
	"bookcasel": "bookcase", "bookcaser": "bookcase", "bookshelf": "bookcase.used",
	"bannerl": "banner", "bannerm": "banner", "bannerr": "banner",
	"crux1": "crux", "crux2": "crux", "crux3": "crux",
	"blindbook": "book1", "bloodbook": "book1", "steeltome": "book1", "booklvr": "book1",
	"book2l": "book2", "book2r": "book2", "skelbook": "book2", "bookstand": "book2",
	"flamelvr": "lever", "bookcandle": "candle2", "storycandle": "candle2", "tbcross": "bcross",
	"mcircle1": "mcircle", "mcircle2": "mcircle",
	"wararmor": "armorstand", "armorstandn": "armorstand.used",
	"warweap": "weaponrack", "weaprack": "weaponrack", "weaponrackn": "weaponrack.used",
}

// PropIconID is IconID("prop", obj) after propFolds.
func PropIconID(obj string) string {
	id := IconID("prop", obj)
	if f, ok := propFolds[strings.TrimPrefix(id, "prop.")]; ok {
		return "prop." + f
	}
	return id
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
