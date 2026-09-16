package defs

// tilesIcons is the name->icon lookup for files/assets/data/icons/tiles.rec, edited at runtime with ./remap_tiles.sh.
var tilesIcons = loadIconRec("tiles")

var (
	IconWhite                    = tilesIcons["IconWhite"]
	IconTargeting                = tilesIcons["IconTargeting"]
	IconGrass                    = tilesIcons["IconGrass"]
	IconDirt                     = tilesIcons["IconDirt"]
	IconMountain                 = tilesIcons["IconMountain"]
	IconWater                    = tilesIcons["IconWater"]
	IconWoodenFence              = tilesIcons["IconWoodenFence"] // first guess
	IconArch                     = tilesIcons["IconArch"]
	IconGrate                    = tilesIcons["IconGrate"]
	IconPillar                   = tilesIcons["IconPillar"] // first guess
	IconLamp                     = tilesIcons["IconLamp"]   // first guess
	IconLava                     = tilesIcons["IconLava"]
	IconWallTorch                = tilesIcons["IconWallTorch"] // first guess
	IconBrazier                  = tilesIcons["IconBrazier"]   // a small flame (131 is a ring icon)
	IconCathedralFloor           = tilesIcons["IconCathedralFloor"]
	IconCatacombsFloor           = tilesIcons["IconCatacombsFloor"]
	IconCatacombsWall            = tilesIcons["IconCatacombsWall"]
	IconCatacombsWallWithShading = tilesIcons["IconCatacombsWallWithShading"]
	IconDungeonRedEarthFloor     = tilesIcons["IconDungeonRedEarthFloor"]
	IconDungeonRockyWall         = tilesIcons["IconDungeonRockyWall"]
	IconCathedralWall            = tilesIcons["IconCathedralWall"]
	IconCathedralWallWithShading = tilesIcons["IconCathedralWallWithShading"]
	IconOpenDoor                 = tilesIcons["IconOpenDoor"]
	IconClosedDoor               = tilesIcons["IconClosedDoor"]
	IconStairsUp                 = tilesIcons["IconStairsUp"]
	IconStairsDown               = tilesIcons["IconStairsDown"]
	IconSign                     = tilesIcons["IconSign"]
	IconDecalBlood1              = tilesIcons["IconDecalBlood1"]
	IconDecalBlood2              = tilesIcons["IconDecalBlood2"]
	IconDecalBlood3              = tilesIcons["IconDecalBlood3"]
	IconDecalBlood4              = tilesIcons["IconDecalBlood4"]
	IconDecalBlood5              = tilesIcons["IconDecalBlood5"]
	IconEffectLightning          = tilesIcons["IconEffectLightning"]
	IconEffectFireSmall          = tilesIcons["IconEffectFireSmall"]
	IconEffectFireBig            = tilesIcons["IconEffectFireBig"]
	IconEffectHit                = tilesIcons["IconEffectHit"]
)
