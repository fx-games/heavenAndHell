package hhm

// The schema is the engine's whole vocabulary: which fields a level carries, and which entities can be placed
// with which fields. An editor reads it to build its forms; the format reader uses it to fill in defaults.

// Field types.
const (
	FString = "string"
	FText   = "text" // a string that may run over several lines
	FInt    = "int"
	FBool   = "bool"
	FTile   = "tile" // an index into the tileset
	FEnum   = "enum"
)

// FieldDef describes one field of a level or an entity.
type FieldDef struct {
	Name    string
	Type    string
	Values  []string // enum fields only
	Default string
	Doc     string
}

// Layers an entity can sit on: Meta holds what the dungeon generators read, Entities what the map loader spawns.
const (
	LayerEntities = "Entities"
	LayerMeta     = "Meta"
)

// EntityKind is one placeable thing.
type EntityKind struct {
	Name   string
	Layer  string
	Doc    string
	Fields []FieldDef
}

// NPCCategories are the roles an NPC can have; a store category also names the shop it opens.
var NPCCategories = []string{"Neutral", "Aggressive", "Store_weapons", "Store_armor", "Store_alchemy", "Store_general"}

// EntityKinds is every entity the engine understands.
var EntityKinds = []EntityKind{
	{Name: "NPC", Layer: LayerEntities, Doc: "a townsperson, or a named monster when Type is Aggressive", Fields: []FieldDef{
		{Name: "Name", Type: FString, Doc: "what the player sees"},
		{Name: "InternalName", Type: FString, Doc: "the vendor type and the icon override key"},
		{Name: "Tile", Type: FTile, Default: "0", Doc: "the icon, unless an override replaces it"},
		{Name: "Type", Type: FEnum, Values: NPCCategories, Default: "Neutral"},
		{Name: "FlipLookDirection", Type: FBool, Default: "false"},
		{Name: "UniqueName", Type: FString, Doc: "the monster id to spawn here, for an Aggressive NPC"},
		{Name: "ASCII", Type: FString, Default: "@", Doc: "the character the ASCII renderer draws"},
	}},
	{Name: "Sign", Layer: LayerEntities, Doc: "a readable sign", Fields: []FieldDef{
		{Name: "Text", Type: FText, Doc: "one line per line"},
	}},
	{Name: "Door", Layer: LayerEntities, Doc: "a door the player can open"},
	{Name: "Stash", Layer: LayerEntities, Doc: "the player's storage chest"},
	{Name: "Shrine", Layer: LayerEntities, Doc: "a shrine"},
	{Name: "Monster", Layer: LayerEntities, Doc: "a monster of that type, placed by the level's population", Fields: []FieldDef{
		{Name: "ID", Type: FString, Doc: "enemies.rec internal_name"},
	}},
	{Name: "Object", Layer: LayerEntities, Doc: "a dungeon object, placed by the level's population", Fields: []FieldDef{
		{Name: "ID", Type: FString, Doc: "the object's id (OBJ_...)"},
		{Name: "Range", Type: FString, Doc: "x1 y1 x2 y2: the part of the map (in megatiles, from the map's corner) a lever opens"},
		{Name: "Opens", Type: FString, Doc: "a set piece whose whole area it opens instead of Range (D1's level 16 levers)"},
		{Name: "Lever", Type: FInt, Default: "0", Doc: "levers sharing a number open their range together"},
	}},
	{Name: "Item", Layer: LayerEntities, Doc: "an item lying here, placed by the level's population", Fields: []FieldDef{
		{Name: "ID", Type: FString, Doc: "items.rec internal_name"},
	}},
	{Name: "Location", Layer: LayerMeta, Doc: "a named spot levels.rec can send the player to", Fields: []FieldDef{
		{Name: "Name", Type: FString},
		{Name: "TargetMap", Type: FString, Doc: "unused by the engine; levels.rec owns where an exit leads"},
		{Name: "TargetLocation", Type: FString, Doc: "unused by the engine"},
	}},
	{Name: "POI", Layer: LayerMeta, Doc: "a spot a generator may build something on"},
	{Name: "NorthConnector", Layer: LayerMeta, Doc: "where another room may join, facing north", Fields: connectorFields},
	{Name: "SouthConnector", Layer: LayerMeta, Doc: "where another room may join, facing south", Fields: connectorFields},
	{Name: "EastConnector", Layer: LayerMeta, Doc: "where another room may join, facing east", Fields: connectorFields},
	{Name: "WestConnector", Layer: LayerMeta, Doc: "where another room may join, facing west", Fields: connectorFields},
}

var connectorFields = []FieldDef{{Name: "ConnectionID", Type: FInt, Default: "0", Doc: "only connectors sharing an id join up"}}

// KindByName returns the entity kind with that name.
func KindByName(name string) (EntityKind, bool) {
	for _, k := range EntityKinds {
		if k.Name == name {
			return k, true
		}
	}
	return EntityKind{}, false
}

// LevelFields describes Props, for an editor's form and for the reader's defaults.
var LevelFields = []FieldDef{
	{Name: "DisplayName", Type: FString, Doc: "the name shown to the player"},
	{Name: "Weight", Type: FInt, Default: "0", Doc: "how often a generator picks this template"},
	{Name: "IsDecoration", Type: FBool, Default: "false", Doc: "a decoration is painted into a room, never placed as one"},
	{Name: "UsesDecoration", Type: FString, Doc: "name prefix of the decorations this room wants"},
	{Name: "SlotID", Type: FInt, Default: "0"},
	{Name: "PlugsIntoSlotID", Type: FInt, Default: "0"},
	{Name: "CanBeLit", Type: FBool, Default: "false"},
}
