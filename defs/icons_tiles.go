package defs

// tilesIcons is the name->icon lookup for files/assets/data/icons/tiles.rec, edited at runtime with ./remap_tiles.sh.
var tilesIcons = loadIconRec("tiles")

var (
	IconWhite                    = tilesIcons["tile.white"]
	IconTargeting                = tilesIcons["tile.targeting"]
	IconGrass                    = tilesIcons["tile.grass"]
	IconDirt                     = tilesIcons["tile.dirt"]
	IconMountain                 = tilesIcons["tile.mountain"]
	IconWater                    = tilesIcons["tile.water"]
	IconWoodenFence              = tilesIcons["tile.wooden_fence"] // first guess
	IconArch                     = tilesIcons["tile.arch"]
	IconGrate                    = tilesIcons["tile.grate"]
	IconPillar                   = tilesIcons["tile.pillar"] // first guess
	IconLamp                     = tilesIcons["tile.lamp"]   // first guess
	IconLava                     = tilesIcons["tile.lava"]
	IconWallTorch                = tilesIcons["tile.wall_torch"] // first guess
	IconBrazier                  = tilesIcons["tile.brazier"]    // a small flame (131 is a ring icon)
	IconCathedralFloor           = tilesIcons["tile.cathedral_floor"]
	IconCatacombsFloor           = tilesIcons["tile.catacombs_floor"]
	IconCatacombsWall            = tilesIcons["tile.catacombs_wall"]
	IconCatacombsWallWithShading = tilesIcons["tile.catacombs_wall_with_shading"]
	IconDungeonRedEarthFloor     = tilesIcons["tile.dungeon_red_earth_floor"]
	IconDungeonRockyWall         = tilesIcons["tile.dungeon_rocky_wall"]
	IconCathedralWall            = tilesIcons["tile.cathedral_wall"]
	IconCathedralWallWithShading = tilesIcons["tile.cathedral_wall_with_shading"]
	IconOpenDoor                 = tilesIcons["tile.open_door"]
	IconClosedDoor               = tilesIcons["tile.closed_door"]
	IconStairsUp                 = tilesIcons["tile.stairs_up"]
	IconStairsDown               = tilesIcons["tile.stairs_down"]
	IconSign                     = tilesIcons["tile.sign"]
	IconDecalBlood1              = tilesIcons["decal.blood1"]
	IconDecalBlood2              = tilesIcons["decal.blood2"]
	IconDecalBlood3              = tilesIcons["decal.blood3"]
	IconDecalBlood4              = tilesIcons["decal.blood4"]
	IconDecalBlood5              = tilesIcons["decal.blood5"]
	IconEffectLightning          = tilesIcons["effect.lightning"]
	IconEffectFireSmall          = tilesIcons["effect.fire_small"]
	IconEffectFireBig            = tilesIcons["effect.fire_big"]
	IconEffectHit                = tilesIcons["effect.hit"]
)
