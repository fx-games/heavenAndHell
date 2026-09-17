package console

import (
	"cmp"
	"image/color"
	"math"
	"slices"
	"strings"
	"unicode"

	mgeom "github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
	"rpg/common"
	"rpg/d1"
	"rpg/defs"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/util"
)

// textui.MapSource

func (c *Console) IsPlayerAndMapInitialized() bool {
	state := c.context.GameState()
	return state != nil && state.GetPlayer() != nil && state.GetMap() != nil
}

func (c *Console) GetMapSize() mgeom.Point {
	return mg(c.context.GameState().GetMap().MapSize())
}

func (c *Console) GetPlayerPosition() mgeom.Point {
	return mg(c.context.GameState().GetPlayer().MapPosition())
}

func (c *Console) IsVisibleToPlayer(loc mgeom.Point) bool {
	pos := rg(loc)
	currentMap := c.context.GameState().GetMap()
	if !currentMap.Contains(pos) {
		return false
	}
	return c.canSeeCell(pos) || c.visibleActorAt(pos) != nil
}

func (c *Console) IsExplored(loc mgeom.Point) bool {
	currentMap := c.context.GameState().GetMap()
	return currentMap.Contains(rg(loc)) && (currentMap.IsExplored(rg(loc)) || c.disableFog)
}

func (c *Console) canSeeCell(pos geometry.Point) bool {
	return c.disableFog || c.context.GameState().GetPlayer().CanSee(pos)
}

// visibleActorAt is the actor the player sees at pos: in view or, with D1 infravision, through walls. Invisible actors stay hidden.
func (c *Console) visibleActorAt(pos geometry.Point) *common.Actor {
	state := c.context.GameState()
	currentMap, player := state.GetMap(), state.GetPlayer()
	if !currentMap.IsActorAt(pos) {
		return nil
	}
	actor := currentMap.ActorAt(pos)
	if !actor.IsVisible() || !c.disableFog && !player.CanPerceive(pos) {
		return nil
	}
	return actor
}

func (c *Console) VisibleIconAt(loc mgeom.Point) textiles.TextIcon {
	pos := rg(loc)
	state := c.context.GameState()
	currentMap := state.GetMap()
	var icon textiles.TextIcon
	if actor := c.visibleActorAt(pos); actor != nil {
		icon = c.actorIcon(actor)
	} else if !c.canSeeCell(pos) {
		return textiles.TextIcon{Char: ' '}
	} else if item, isItemAt := currentMap.TryGetItemAt(pos); isItemAt && item.IsDrawn() && (!item.IsHidden() || state.GetFlags().IsSet(string(common.FlagHasTrueSight))) {
		icon = textiles.TextIcon{Char: itemGlyph(item), Fg: item.TintColor()}
	} else if objectIcon, isObject := c.objectIconAt(pos); isObject {
		icon = objectIcon
	} else if currentMap.IsDownedActorAt(pos) {
		icon = textiles.TextIcon{Char: '%', Fg: currentMap.DownedActorAt(pos).GetColor()}
	} else {
		icon = c.tileIcon(pos)
	}
	if !c.disableFog {
		icon.Fg = state.ApplyLighting(pos, icon.Fg, c.ticks)
	}
	icon.Bg = color.RGBA{A: 255}
	return icon
}

func (c *Console) ExploredIconAt(loc mgeom.Point) textiles.TextIcon {
	pos := rg(loc)
	icon, isObject := c.objectIconAt(pos)
	if !isObject {
		icon = c.tileIcon(pos)
	}
	icon.Bg = color.RGBA{A: 255}
	return icon
}

func (c *Console) actorIcon(actor *common.Actor) textiles.TextIcon {
	glyph := actor.AsciiIcon(c.ticks)
	if glyph == 0 {
		glyph = unicode.ToUpper([]rune(actor.Name() + "?")[0])
	}
	fg := actor.GetColor()
	if actor.IsHurt() {
		if lerped, ok := util.LerpColor(defs.Red, fg, actor.GetHurtTicksAsPercentage()).(color.RGBA); ok {
			fg = lerped
		}
	}
	return textiles.TextIcon{Char: glyph, Fg: fg, Bg: color.RGBA{A: 255}}
}

func (c *Console) tileIcon(pos geometry.Point) textiles.TextIcon {
	currentMap := c.context.GameState().GetMap()
	tile := currentMap.GetCell(pos).TileType
	icon := textiles.TextIcon{Char: '#', Fg: defs.OffWhite}
	switch {
	case tile.Special == gridmap.SpecialTileWater:
		icon = textiles.TextIcon{Char: '~', Fg: c.context.GameState().GetWaterColor()}
	case tile.Special == gridmap.SpecialTileDamaging:
		icon = textiles.TextIcon{Char: '~', Fg: defs.Red}
	case tile.Special == gridmap.SpecialTileForest:
		icon = textiles.TextIcon{Char: ';', Fg: defs.Brown}
	case tile.Special == gridmap.SpecialTileMountain:
		icon = textiles.TextIcon{Char: '^', Fg: defs.LightGray}
	case tile.Special == gridmap.SpecialTileVoid:
		icon = textiles.TextIcon{Char: ' '}
	case tile.Special == gridmap.SpecialTileStairsUp:
		icon = textiles.TextIcon{Char: '<', Fg: defs.White}
	case tile.Special == gridmap.SpecialTileStairsDown:
		icon = textiles.TextIcon{Char: '>', Fg: defs.White}
	case tile.IsWalkable:
		icon = textiles.TextIcon{Char: '.', Fg: defs.DarkGray}
		if _, hasDecal := currentMap.GetDecal(pos); hasDecal {
			icon.Fg = color.RGBA{R: 140, G: 20, B: 20, A: 255}
		}
	}
	return icon
}

func itemGlyph(item *common.Item) rune {
	d := item.D1()
	switch {
	case item.IsGold():
		return '$'
	case item.IsPotion():
		return '!'
	case item.IsScroll():
		return '?'
	case d != nil && d.MiscID == "IMISC_BOOK":
		return '+'
	case item.IsRing():
		return '='
	case item.IsAmulet():
		return '"'
	case d != nil && d.Type == d1.ITypeStaff:
		return '_'
	case item.IsRangedWeapon():
		return '}'
	case item.IsWeapon():
		return ')'
	case item.IsArmor():
		return '['
	}
	return '&'
}

// objectGlyphs pick an object's glyph and color by a word in its name, first match wins.
var objectGlyphs = []struct {
	word  string
	glyph rune
	color color.RGBA
}{
	{"portal", 'O', color.RGBA{R: 80, G: 120, B: 255, A: 255}},
	{"chest", '=', color.RGBA{R: 220, G: 180, B: 60, A: 255}},
	{"barrel", '0', defs.Brown},
	{"sarcophagus", '&', defs.LightGray},
	{"shrine", '_', color.RGBA{R: 80, G: 220, B: 220, A: 255}},
	{"altar", '_', color.RGBA{R: 200, G: 80, B: 200, A: 255}},
	{"book", '+', color.RGBA{R: 200, G: 80, B: 200, A: 255}},
	{"tome", '+', color.RGBA{R: 200, G: 80, B: 200, A: 255}},
	{"lever", '/', defs.LightGray},
	{"fountain", '{', color.RGBA{R: 80, G: 120, B: 255, A: 255}},
	{"pool", '{', color.RGBA{R: 80, G: 160, B: 80, A: 255}},
	{"spring", '{', color.RGBA{R: 80, G: 120, B: 255, A: 255}},
	{"cross", '+', defs.Red},
	{"fire", '^', defs.Red},
	{"flame", '^', defs.Red},
	{"candle", 'i', color.RGBA{R: 255, G: 220, B: 120, A: 255}},
	{"rack", '|', defs.LightGray},
	{"stand", '|', defs.LightGray},
	{"armor", '[', defs.LightGray},
	{"skull", '%', defs.OffWhite},
	{"skeleton", '%', defs.OffWhite},
	{"body", '%', defs.Red},
	{"hero", '%', defs.OffWhite},
	{"stash", '=', color.RGBA{R: 220, G: 180, B: 60, A: 255}},
	{"sign", '"', defs.Brown},
	{"circle", 'o', color.RGBA{R: 200, G: 80, B: 200, A: 255}},
	{"water", '~', color.RGBA{R: 80, G: 120, B: 255, A: 255}},
	{"cauldron", 'u', defs.LightGray},
	{"banner", '|', defs.Red},
	{"statue", '&', defs.LightGray},
	{"mushroom", '"', color.RGBA{R: 160, G: 120, B: 200, A: 255}},
	{"pedestal", '_', defs.Red},
}

func (c *Console) objectIconAt(pos geometry.Point) (textiles.TextIcon, bool) {
	currentMap := c.context.GameState().GetMap()
	if !currentMap.IsObjectAt(pos) {
		return textiles.TextIcon{}, false
	}
	object := currentMap.ObjectAt(pos)
	if door, isDoor := object.(*common.Door); isDoor {
		if door.IsClosed() {
			return textiles.TextIcon{Char: '+', Fg: defs.Brown}, true
		}
		return textiles.TextIcon{Char: '\'', Fg: defs.Brown}, true
	}
	name := strings.ToLower(object.Name())
	if name == "" { // wall lights and other fixtures without a name draw as their tile
		return textiles.TextIcon{}, false
	}
	icon := textiles.TextIcon{Char: []rune(name)[0], Fg: defs.OffWhite}
	for _, entry := range objectGlyphs {
		if strings.Contains(name, entry.word) {
			icon = textiles.TextIcon{Char: entry.glyph, Fg: entry.color}
			break
		}
	}
	if d1Object, isD1 := object.(*common.D1Object); isD1 && d1Object.TintColor() != defs.White {
		icon.Fg = d1Object.TintColor() // opened or used
	}
	return icon, true
}

// describe names what the player sees at a map position, like the tile UI's hover info.
func (c *Console) describe(loc mgeom.Point) string {
	pos := rg(loc)
	currentMap := c.context.GameState().GetMap()
	if !currentMap.Contains(pos) {
		return " "
	}
	if actor := c.visibleActorAt(pos); actor != nil {
		return actor.Name()
	}
	if !c.canSeeCell(pos) {
		return " "
	}
	switch {
	case currentMap.IsItemAt(pos):
		return currentMap.ItemAt(pos).Name()
	case currentMap.IsObjectAt(pos) && currentMap.ObjectAt(pos).Name() != "":
		return currentMap.ObjectAt(pos).Name()
	}
	return " "
}

func (c *Console) visibleEnemiesByDistance() []*common.Actor {
	return c.byDistance(c.context.GameState().GetVisibility().GetVisibleEnemies())
}

func (c *Console) visibleActorsByDistance() []*common.Actor {
	var actors []*common.Actor
	for actor := range c.context.GameState().GetVisibility().GetVisibleActors() {
		actors = append(actors, actor)
	}
	return c.byDistance(actors)
}

// byDistance orders actors nearest first. The callers iterate a map, so equal distances are broken by position to
// keep the order stable from frame to frame.
func (c *Console) byDistance(actors []*common.Actor) []*common.Actor {
	playerPos := c.context.GameState().GetPlayer().MapPosition()
	slices.SortFunc(actors, func(a, b *common.Actor) int {
		aPos, bPos := a.MapPosition(), b.MapPosition()
		return cmp.Or(
			aPos.ManhattanDistanceTo(playerPos)-bPos.ManhattanDistanceTo(playerPos),
			aPos.Y-bPos.Y,
			aPos.X-bPos.X,
		)
	})
	return actors
}

// Animations

// animationCanvas is the common.TileRenderer the game's animations draw on: projectiles become '*', damage numbers text.
type animationCanvas map[mgeom.Point]textiles.TextIcon

func (c *Console) animationIcons() map[mgeom.Point]textiles.TextIcon {
	canvas := animationCanvas{}
	c.context.Animator().Draw(canvas)
	return canvas
}

func (a animationCanvas) DrawOnMap(mapPos geometry.Point, _ int32, drawColor color.Color) {
	a[mg(mapPos)] = textiles.TextIcon{Char: '*', Fg: toRGBA(drawColor)}
}

func (a animationCanvas) DrawOnMapF(pos geometry.PointF, icon int32, drawColor color.Color) {
	a.DrawOnMap(geometry.Point{X: int(math.Round(pos.X)), Y: int(math.Round(pos.Y))}, icon, drawColor)
}

func (a animationCanvas) DrawStringOnMapWithOffset(mapPos, _ geometry.Point, text string, drawColor color.Color) {
	rgba := toRGBA(drawColor)
	if rgba.R == 0 && rgba.G == 0 && rgba.B == 0 { // the drop shadow of damage numbers
		return
	}
	for i, r := range text {
		a[mgeom.Point{X: mapPos.X + i, Y: mapPos.Y - 1}] = textiles.TextIcon{Char: r, Fg: rgba}
	}
}

func (a animationCanvas) MeasureString(text string) (float64, float64) {
	return float64(len(text)), 1
}

func (a animationCanvas) GetScaledTileSize() geometry.PointF {
	return geometry.PointF{X: 1, Y: 1}
}

func toRGBA(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()
	return color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}
