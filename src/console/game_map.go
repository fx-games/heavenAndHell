package console

import (
	"cmp"
	"image/color"
	"math"
	"slices"
	"strings"
	"unicode"

	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/recfile"
	"rpg/src/rules"
	"rpg/src/util"

	mgeom "github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
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
	actor, ok := currentMap.TryGetActorAt(pos)
	if !ok {
		return nil
	}
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
	} else if downed, ok := currentMap.TryGetDownedActorAt(pos); ok {
		icon = textiles.TextIcon{Char: '%', Fg: downed.GetColor()}
	} else {
		icon = c.tileIcon(pos)
	}
	if !c.disableFog && !(c.visibleActorAt(pos) != nil && state.LitByVision(pos)) {
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
	d := item.Roll()
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
	case d != nil && d.Type == rules.ITypeStaff:
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

// objectGlyph is a glyphs.rec record: how ASCII mode draws an object of one id.
type objectGlyph struct {
	glyph rune
	color color.RGBA
}

// loadObjectGlyphs reads the mod's data/ascii/glyphs.rec, by object id.
func loadObjectGlyphs() map[string]objectGlyph {
	out := map[string]objectGlyph{}
	for _, r := range recfile.Read(mustRead("assets/data/ascii/glyphs.rec")) {
		m := r.ToMap("")
		if id, g := m.GetStringOrDefault("id", ""), []rune(m.GetStringOrDefault("glyph", "")); id != "" && len(g) > 0 {
			out[id] = objectGlyph{g[0], util.GetColorFromRGBString(m.GetStringOrDefault("color", ""))}
		}
	}
	return out
}

func (c *Console) objectIconAt(pos geometry.Point) (textiles.TextIcon, bool) {
	currentMap := c.context.GameState().GetMap()
	object, ok := currentMap.TryGetObjectAt(pos)
	if !ok {
		return textiles.TextIcon{}, false
	}
	icon := textiles.TextIcon{Fg: defs.OffWhite}
	if g, ok := c.objectGlyphs[object.ID()]; ok {
		icon = textiles.TextIcon{Char: g.glyph, Fg: g.color}
	} else if name := strings.ToLower(object.Name()); name != "" {
		icon.Char = []rune(name)[0]
	} else { // wall lights and other fixtures without a name draw as their tile
		return textiles.TextIcon{}, false
	}
	if d1Object, isD1 := object.(*common.Prop); isD1 && d1Object.TintColor() != defs.White {
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
	if item, ok := currentMap.TryGetItemAt(pos); ok {
		return item.Name()
	}
	if object, ok := currentMap.TryGetObjectAt(pos); ok && object.Name() != "" {
		return object.Name()
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
