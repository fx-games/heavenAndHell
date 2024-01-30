package game

import (
	"path"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ldtk_go"
	"rpg/renderer"
	"strings"
)

func (g *GameInstance) NewGridMapFromLDTK(mapName string) (*gridmap.GridMap[*common.Actor, common.Item, common.Object], renderer.TextureAtlas) {
	ldtkMapProject := g.ldtkProject
	worldTileset := ldtkMapProject.TilesetByIdentifier("World")

	currentMap := ldtkMapProject.LevelByIdentifier(mapName)
	locations := loadLocations(currentMap)

	mapDisplayName := currentMap.PropertyByIdentifier("DisplayName").AsString()

	environmentLayer := currentMap.LayerByIdentifier("Tiles")
	//metaLayer := currentMap.LayerByIdentifier("Meta")
	npcLayer := currentMap.LayerByIdentifier("Entities")

	loadedMap := gridmap.NewEmptyMap[*common.Actor, common.Item, common.Object](environmentLayer.CellWidth, environmentLayer.CellHeight)
	loadedMap.SetCardinalMovementOnly(g.configuration.CardinalMovementOnly)
	loadedMap.SetName(mapName)
	loadedMap.SetDisplayName(mapDisplayName)
	loadedMap.SetNamedLocations(locations)

	for _, tile := range environmentLayer.Tiles {
		pos, tileType := MapTileFromLDTK(worldTileset, environmentLayer, tile)
		loadedMap.SetTile(pos, tileType)
	}

	for _, entity := range npcLayer.Entities {
		typeOfEntity := entity.Identifier
		posX, posY := npcLayer.ToGridPosition(entity.Position[0], entity.Position[1])
		pos := geometry.Point{X: posX, Y: posY}
		if typeOfEntity == "NPC" {
			addActorFromEntity(ldtkMapProject, loadedMap, entity, pos)
		} else if typeOfEntity == "Sign" {
			signText := entity.PropertyByIdentifier("Text").AsString()
			textLines := strings.Split(signText, "\n")
			g.addSignFromEntity(loadedMap, textLines, pos)
		} else if typeOfEntity == "Door" {
			loadedMap.AddObject(common.NewDoor(common.IconOpenDoor, common.IconClosedDoor), pos)
		}

	}

	textureAtlas := renderer.NewTextureAtlas(path.Join("assets", worldTileset.Path), worldTileset.GridSize, worldTileset.GridSize)
	return loadedMap, textureAtlas
}

func (g *GameInstance) addSignFromEntity(loadedMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], inscription []string, pos geometry.Point) {
	sign := g.newSign(inscription)
	loadedMap.AddObject(sign, pos)
}

func addActorFromEntity(ldtkMapProject *ldtk_go.Project, loadedMap *gridmap.GridMap[*common.Actor, common.Item, common.Object], entity *ldtk_go.Entity, pos geometry.Point) {
	icon, _ := resolveLDTKIcon(ldtkMapProject, entity.PropertyByIdentifier("Tile"))

	flipLook := entity.PropertyByIdentifier("FlipLookDirection").AsBool()

	actorName := entity.PropertyByIdentifier("Name").AsString()
	actor := common.NewActor(icon)
	actor.SetName(actorName)
	actor.SetDrawPosition(pos.ToPointF())

	if !entity.PropertyByIdentifier("InternalName").IsNull() {
		actor.SetInternalName(entity.PropertyByIdentifier("InternalName").AsString())
	}

	npcTypeProp := entity.PropertyByIdentifier("Type").AsString()
	if strings.HasPrefix(npcTypeProp, "Vendor") {
		actor.SetVendorType(common.VendorType(strings.ToLower(npcTypeProp)))
		actor.SetNoCorpse()
		actor.SetDeathStringFormats([]string{
			"%s teleported away at the last second.",
		})
	} else if strings.HasPrefix(npcTypeProp, "Neutral") {
		actor.SetNeutral()
	} else if strings.HasPrefix(npcTypeProp, "Aggressive") {
		actor.SetHostile()
	}

	loadedMap.AddActor(actor, pos)
	actor.SetFlipLook(flipLook)
}

func resolveLDTKIcon(ldtkMapProject *ldtk_go.Project, prop *ldtk_go.Property) (int32, ldtk_go.EnumSet) {
	tileData := prop.Value.(map[string]interface{})
	tileset := ldtkMapProject.TilesetByUID(int(tileData["tilesetUid"].(float64)))
	tilesetWidth := tileset.Width / tileset.GridSize
	atlasX := int(tileData["x"].(float64) / float64(tileset.GridSize))
	atlasY := int(tileData["y"].(float64) / float64(tileset.GridSize))
	textureIndex := atlasY*tilesetWidth + atlasX
	enumsForTile := tileset.EnumsForTile(textureIndex)
	return int32(textureIndex), enumsForTile
}
func loadLocations(level *ldtk_go.Level) []gridmap.NamedLocation {
	locations := make([]gridmap.NamedLocation, 0)
	metaLayer := level.LayerByIdentifier("Meta")
	for _, entity := range metaLayer.Entities {
		if entity.Identifier != "Location" {
			continue
		}
		nameOfLocation := entity.PropertyByIdentifier("Name").AsString()
		posX, posY := metaLayer.ToGridPosition(entity.Position[0], entity.Position[1])
		pos := geometry.Point{X: posX, Y: posY}
		if nameOfLocation != "" {
			locations = append(locations, gridmap.NamedLocation{
				LocationName: nameOfLocation,
				Pos:          pos,
			})
		}
	}
	return locations
}
