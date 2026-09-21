package defs

// tilesIcons is the name->icon lookup for files/assets/data/icons/tiles.rec, edited at runtime with ./remap_tiles.sh.
var tilesIcons map[string]int32

var (
	IconWhite                    int32
	IconTargeting                int32
	IconGrass                    int32
	IconDirt                     int32
	IconMountain                 int32
	IconWater                    int32
	IconWoodenFence              int32
	IconArch                     int32
	IconGrate                    int32
	IconPillar                   int32
	IconLamp                     int32
	IconLava                     int32
	IconWallTorch                int32
	IconBrazier                  int32
	IconCathedralFloor           int32
	IconCatacombsFloor           int32
	IconCatacombsWall            int32
	IconCatacombsWallWithShading int32
	IconDungeonRedEarthFloor     int32
	IconDungeonRockyWall         int32
	IconCathedralWall            int32
	IconCathedralWallWithShading int32
	IconOpenDoor                 int32
	IconClosedDoor               int32
	IconStairsUp                 int32
	IconStairsDown               int32
	IconSign                     int32
	IconDecalBlood1              int32
	IconDecalBlood2              int32
	IconDecalBlood3              int32
	IconDecalBlood4              int32
	IconDecalBlood5              int32
	IconEffectLightning          int32
	IconEffectFireSmall          int32
	IconEffectFireBig            int32
	IconEffectHit                int32
	IconEffectArrow              int32
)

func tileIcon(name string) int32 { return namedIcon(tilesIcons, name) }

func loadTilesIcons() {
	tilesIcons = loadIconRec("tiles")
	IconWhite = tileIcon("tile.white")
	IconTargeting = tileIcon("tile.targeting")
	IconGrass = tileIcon("tile.grass")
	IconDirt = tileIcon("tile.dirt")
	IconMountain = tileIcon("tile.mountain")
	IconWater = tileIcon("tile.water")
	IconWoodenFence = tileIcon("tile.wooden_fence") // first guess
	IconArch = tileIcon("tile.arch")
	IconGrate = tileIcon("tile.grate")
	IconPillar = tileIcon("tile.pillar") // first guess
	IconLamp = tileIcon("tile.lamp")     // first guess
	IconLava = tileIcon("tile.lava")
	IconWallTorch = tileIcon("tile.wall_torch") // first guess
	IconBrazier = tileIcon("tile.brazier")      // a small flame (131 is a ring icon)
	IconCathedralFloor = tileIcon("tile.cathedral_floor")
	IconCatacombsFloor = tileIcon("tile.catacombs_floor")
	IconCatacombsWall = tileIcon("tile.catacombs_wall")
	IconCatacombsWallWithShading = tileIcon("tile.catacombs_wall_with_shading")
	IconDungeonRedEarthFloor = tileIcon("tile.dungeon_red_earth_floor")
	IconDungeonRockyWall = tileIcon("tile.dungeon_rocky_wall")
	IconCathedralWall = tileIcon("tile.cathedral_wall")
	IconCathedralWallWithShading = tileIcon("tile.cathedral_wall_with_shading")
	IconOpenDoor = tileIcon("tile.open_door")
	IconClosedDoor = tileIcon("tile.closed_door")
	IconStairsUp = tileIcon("tile.stairs_up")
	IconStairsDown = tileIcon("tile.stairs_down")
	IconSign = tileIcon("tile.sign")
	IconDecalBlood1 = tileIcon("decal.blood1")
	IconDecalBlood2 = tileIcon("decal.blood2")
	IconDecalBlood3 = tileIcon("decal.blood3")
	IconDecalBlood4 = tileIcon("decal.blood4")
	IconDecalBlood5 = tileIcon("decal.blood5")
	IconEffectLightning = tileIcon("effect.lightning")
	IconEffectFireSmall = tileIcon("effect.fire_small")
	IconEffectFireBig = tileIcon("effect.fire_big")
	IconEffectHit = tileIcon("effect.hit")
	IconEffectArrow = tileIcon("effect.arrow")
}
