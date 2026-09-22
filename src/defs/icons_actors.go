package defs

// actorsIcons is the name->icon lookup for files/mods/<mod>/data/icons/actors.rec, edited at runtime with scripts/remap_actors.sh.
var actorsIcons map[string]int32

// The playable classes are a mod's content, not the engine's, so their icons are NOT in this fixed list: they
// resolve by name from the class a mod declared (classes.rec -> ClassIcons). An engine that names warrior,
// rogue and sorcerer here demands one game's class roster of every mod.
var (
	IconCorpse int32
)

func actorIcon(name string) int32 { return namedIcon(actorsIcons, name) }

func loadActorsIcons() {
	actorsIcons = loadIconRec("actors")
	IconCorpse = actorIcon("corpse.default")
}
