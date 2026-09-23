package common

import (
	"fmt"
	"path"
	"rpg/src/defs"
	"rpg/src/dungeon"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/hhm"
	"rpg/src/renderer"
	"rpg/src/util"
	"strings"
)

// loadDrawnLevel is the "drawn" generator: level lvl as drawn under template in the mod's map set. Its named
// locations come from the drawing; where they lead is levels.rec's business (exits), not the drawing's.
func (s *State) loadDrawnLevel(lvl dungeon.Level, template string) *gridmap.GridMap[*Actor, *Item, Object] {
	maps := s.drawnMaps
	worldTileset := maps.Tileset
	mapName := lvl.Map

	currentMap := maps.Level(template)
	if currentMap == nil {
		panic(fmt.Sprintf("levels.rec: %s: the mod's maps have no level %q", lvl.Map, template))
	}

	loadedMap := gridmap.NewEmptyMap[*Actor, *Item, Object](currentMap.Width, currentMap.Height)
	loadedMap.SetName(mapName)
	loadedMap.SetDisplayName(currentMap.Props.DisplayName)
	loadedMap.SetNamedLocations(loadLocations(currentMap))

	for y := 0; y < currentMap.Height; y++ {
		for x := 0; x < currentMap.Width; x++ {
			if id := currentMap.TileAt(x, y); id != hhm.Empty {
				loadedMap.SetTile(geometry.Point{X: x, Y: y}, TileFromDrawing(worldTileset, id))
			}
		}
	}

	for _, entity := range currentMap.Entities {
		pos := geometry.Point{X: entity.X, Y: entity.Y}
		switch entity.Kind {
		case "NPC":
			// a hostile NPC is one of the mod's monsters, named as room templates name theirs
			if entity.Str("Type") == "Aggressive" && entity.Str("UniqueName") != "" {
				if a := s.factory.NewMonsterByID(entity.Str("UniqueName")); a != nil {
					a.SetDrawPosition(pos.ToPointF())
					loadedMap.AddActor(a, pos)
				}
				continue
			}
			addActorFromEntity(loadedMap, entity, pos)
		case "Sign":
			s.addSignFromEntity(loadedMap, strings.Split(entity.Str("Text"), "\n"), pos)
		case "Door":
			loadedMap.AddObject(NewDoor(defs.IconOpenDoor, defs.IconClosedDoor), pos)
		case "Stash":
			loadedMap.AddObject(s.factory.NewStash(), pos)
		}
	}

	s.worldTexture = renderer.NewTextureAtlas(path.Join("assets", worldTileset.Image), worldTileset.GridSize, worldTileset.GridSize)
	return loadedMap
}

func (s *State) addSignFromEntity(loadedMap *gridmap.GridMap[*Actor, *Item, Object], inscription []string, pos geometry.Point) {
	sign := s.factory.NewSign(inscription)
	loadedMap.AddObject(sign, pos)
}

func addActorFromEntity(loadedMap *gridmap.GridMap[*Actor, *Item, Object], entity *hhm.Entity, pos geometry.Point) {
	icon := int32(entity.Int("Tile"))

	actorName := entity.Str("Name")
	// every NPC is a townsperson: its internal name is its vendor type
	internalName := entity.Str("InternalName")
	if remapped, ok := defs.IconOK(defs.IconID("npc", internalName)); ok { // scripts/remap_actors.sh overrides the tile the map draws
		icon = remapped
	}
	actor := NewActor(icon)
	actor.SetName(actorName)
	ascii := int32('@')
	if a := entity.Str("ASCII"); a != "" {
		ascii = int32(a[0])
	}
	actor.SetAsciiIcon(ascii)
	actor.SetCP437Icon(int32(util.UnicodeToCP437Byte(ascii)))
	actor.SetUnicodeIcon(icon)

	actor.SetColor(defs.White)

	actor.SetDrawPosition(pos.ToPointF())

	actor.SetInternalName(internalName)
	actor.SetVendorType(VendorType(internalName))
	actor.SetNoCorpse()
	actor.SetDeathStringFormats([]string{
		"%s teleported away at the last second.",
	})

	loadedMap.AddActor(actor, pos)
	actor.SetFlipLook(entity.Bool("FlipLookDirection"))
}

func loadLocations(level *hhm.Level) []gridmap.NamedLocation {
	locations := make([]gridmap.NamedLocation, 0)
	for _, entity := range level.Entities {
		if entity.Kind != "Location" || entity.Str("Name") == "" {
			continue
		}
		locations = append(locations, gridmap.NamedLocation{
			LocationName: entity.Str("Name"),
			Pos:          geometry.Point{X: entity.X, Y: entity.Y},
		})
	}
	return locations
}
