package gui

import (
	"image/color"
	"rpg/src/common"
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/gridmap"
	"rpg/src/renderer"
	"rpg/src/stats"
	"rpg/src/util"
	"slices"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

func (u *UIController) Draw(screen *ebiten.Image) {
	u.tileRenderer.SetRenderTarget(screen)

	u.mapRenderer.Draw(u.worldTicks)

	u.drawActors()

	u.context.Animator().Draw(u.mapRenderer) // should be some "animator"

	// the cursor square belongs to a running game: not under the title screen or the start menu
	if !u.IsMouseCapturedByUI() && u.title == nil && !u.sidebarHiddenForMenu {
		if u.targeter != nil {
			u.targeter.Draw(u.mapRenderer)
		} else {
			// default map selection rectangle
			u.mapRenderer.DrawOnMap(u.mousePosOnMap, defs.IconTargeting, color.White)
		}
	}

	u.captureThumbnail(screen) // before the UI: a save slot's screenshot shows the world, not the menus

	// While the ceremony runs it covers the UI too - the sidebar has no business staying bright while the
	// screen bleeds. Once the menu is up, the same overlay is the backdrop it sits on.
	if u.death != nil && u.death.menuOpen {
		u.drawDeathScreen(screen)
	}
	if alpha := u.uiFadeAlpha(); alpha < 1 {
		u.drawUIWithAlpha(screen, alpha)
	} else {
		u.drawUI()
	}

	if u.message.ticksLeft > 0 {
		const pad = 3.0
		w, h := u.tileRenderer.MeasureString(u.message.GetText())
		u.tileRenderer.DrawColoredRect(geometry.Point{X: 0, Y: int(u.message.GetHeight() - h - pad)},
			geometry.Point{X: int(w + 2*pad), Y: int(h + 2*pad)}, color.RGBA{A: 160})

		u.tileRenderer.DrawTTFOnScreenWithColorCodes(1, u.message.GetHeight(), u.message.GetColorcodedText())
		u.message.ticksLeft--
	}

	if u.currentTooltip != nil && !u.currentTooltip.IsNull() && u.ticksUntilTooltipAppears == 0 {
		u.currentTooltip.Draw()
	}

	if u.death != nil && !u.death.menuOpen {
		u.drawDeathScreen(screen)
	}

	if u.title != nil {
		u.drawTitleScreen()
	}
}

// uniqueSweepShaderSrc draws a unique monster's sprite with a specular sheen: a diagonal band of light that
// sweeps across the sprite once every uniqueSweepIntervalTicks, then is gone until the next pass. The highlight
// is added on top of the lit sprite rather than blended into it, so it reads as light catching the surface and
// stays visible in dim rooms, where a color blend would just be swallowed by the ambient lighting tint.
//
// The pixel-unit directive is required: in Kage's default texel mode srcPos and imageSrc0Origin are normalized
// against the internal GPU atlas, so the local 0..1 coordinates below would be meaningless.
const uniqueSweepShaderSrc = `//kage:unit pixels

package main

var Time float
var IntervalTicks float
var SweepFraction float
var Tint vec4

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	c := imageSrc0UnsafeAt(srcPos)
	lit := c * Tint

	local := (srcPos - imageSrc0Origin()) / imageSrc0Size()
	acrossSprite := (local.x + local.y) * 0.5 // 0 at the top-left corner, 1 at the bottom-right one

	sheen := 0.0
	phase := mod(Time, IntervalTicks) / IntervalTicks
	if phase < SweepFraction {
		center := 1.2 - (phase/SweepFraction)*1.4 // travels past both corners, so the sweep enters and exits cleanly
		falloff := clamp(1.0-abs(acrossSprite-center)/0.16, 0.0, 1.0)
		sheen = falloff * falloff // squared: a tight bright core with soft edges, like a highlight
	}

	highlight := vec3(1.0, 0.92, 0.6) * sheen * lit.a // scaled by coverage to stay premultiplied, and so it fades out with the sprite
	return vec4(lit.rgb+highlight, lit.a)
}
`

var uniqueSweepShader = mustCompileShader(uniqueSweepShaderSrc)

// stoneShader draws a Stone Cursed actor's lit sprite in grayscale.
var stoneShader = mustCompileShader(`//kage:unit pixels

package main

var Tint vec4

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	lit := imageSrc0UnsafeAt(srcPos) * Tint
	return vec4(vec3(dot(lit.rgb, vec3(0.299, 0.587, 0.114))), lit.a)
}
`)

func mustCompileShader(src string) *ebiten.Shader {
	shader, err := ebiten.NewShader([]byte(src))
	if err != nil {
		panic(err)
	}
	return shader
}

const uniqueSweepIntervalTicks = 150
const uniqueSweepFraction = 0.35

// drawUniqueSpecularSheen makes a unique monster hard to miss: a band of light sweeps diagonally across its
// sprite every uniqueSweepIntervalTicks ticks, then is gone until the next pass. It replaces the normal tinted
// sprite draw for unique actors (not an overlay on top of one) so there's no second, unlit copy of the sprite
// sitting on top and hiding the lighting/hurt tint.
func (u *UIController) drawUniqueSpecularSheen(actor *common.Actor, screenX, screenY float64, atlas renderer.TextureAtlas, icon int32, scale geometry.PointF, tint color.Color, flippedX bool) {
	u.tileRenderer.DrawScaledTileWithShader(screenX, screenY, atlas, icon, scale, tint, flippedX, uniqueSweepShader, map[string]interface{}{
		"Time":          float32(u.worldTicks + actorPhase(actor)%uniqueSweepIntervalTicks),
		"IntervalTicks": float32(uniqueSweepIntervalTicks),
		"SweepFraction": float32(uniqueSweepFraction),
	})
}

func (u *UIController) drawActors() {
	visibleMapOnScreen := u.mapRenderer.GetVisibleMap()
	state := u.context.GameState()
	player := state.GetPlayer()

	enemyColor := defs.White
	actorAtlas := u.GetCurrentAtlas()

	for _, actor := range slices.Concat(state.GetMap().Actors(), state.Dying()) {
		if !visibleMapOnScreen.Contains(actor.MapPosition()) || !player.CanPerceive(actor.MapPosition()) {
			continue
		}
		if !actor.IsVisible() {
			continue
		}

		actorColor := enemyColor
		if u.renderingMode != GraphicRendering || actor.Monster() != nil { // decision 8: D1 variants are tinted sprites
			actorColor = actor.GetColor()
		}
		var drawColor color.Color
		drawColor = state.ApplyLighting(actor.MapPosition(), actorColor, u.worldTicks)
		if state.LitByVision(actor.MapPosition()) {
			drawColor = actorColor
		}

		if actor.IsHurt() && !actor.VisualsPending() {
			percentage := actor.GetHurtTicksAsPercentage() // goes from 1.0 to 0.0
			drawColor = util.LerpColor(defs.Red, actorColor, percentage)
		}
		if u.death != nil && actor == player { // his death is a scene of its own, not a sprite swap
			u.drawDyingHero(actor, actorAtlas, drawColor)
			continue
		}
		if actor.HasStatusEffect(common.StatusStone) { // petrified: frozen and colorless
			screenPos := u.mapRenderer.MapFloatToScreen(actor.DrawPosition())
			u.tileRenderer.DrawScaledTileWithShader(float64(screenPos.X), float64(screenPos.Y), actorAtlas, u.getActorIcon(actor, 0), geometry.PointF{X: 1, Y: 1}, drawColor, actor.FlipX() && u.configuration.FlipActorsHorizontally, stoneShader, nil)
		} else if u.configuration.ActorBobbingEnabled {
			u.drawActorWithBobbingAnimation(actor, actorAtlas, drawColor, actor.IsUnique())
		} else {
			screenPos := u.mapRenderer.MapFloatToScreen(actor.DrawPosition())
			flippedX := actor.FlipX() && u.configuration.FlipActorsHorizontally
			icon := u.getActorIcon(actor, u.worldTicks)
			if actor.IsUnique() {
				u.drawUniqueSpecularSheen(actor, float64(screenPos.X), float64(screenPos.Y), actorAtlas, icon, geometry.PointF{X: 1, Y: 1}, drawColor, flippedX)
			} else {
				u.tileRenderer.DrawDefaultScaleTileWithFlip(float64(screenPos.X), float64(screenPos.Y), actorAtlas, icon, drawColor, flippedX)
			}
		}
	}

	for _, actor := range state.GetMap().Actors() {
		if !actor.IsVisible() || !visibleMapOnScreen.Contains(actor.MapPosition()) || !player.CanPerceive(actor.MapPosition()) {
			continue
		}
		if u.death != nil && actor == player { // an empty health bar over the corpse spoils the scene
			continue
		}
		if atMax := actor.GetStats().GetResource(stats.Health).IsAtMax(); !atMax && !actor.VisualsPending() || actor.VisualsPending() && actor.ShownHealth() < actor.GetStats().GetAttribute(stats.MaxHealth) {
			u.drawHealthBar(actor)
		}
	}
}

func (u *UIController) drawActorWithBobbingAnimation(actor *common.Actor, atlas renderer.TextureAtlas, drawColor color.Color, isUnique bool) {
	screenPos := u.mapRenderer.MapFloatToScreen(actor.DrawPosition())
	tileSize := u.mapRenderer.GetScaledTileSize()
	ticksForAnimation := u.worldTicks + actorPhase(actor)%3600
	percentage := util.GetPercentageFromTick(ticksForAnimation, 1) // -> [0.0..1.0)  but we want [0.0..1.0..0.0]
	percentage *= 2
	if percentage > 1 {
		percentage = 2 - percentage
	}
	scaleUp := geometry.PointF{X: 1, Y: 1}
	scaleDown := geometry.PointF{X: 1.05, Y: 0.95}
	currentScale := util.LerpPointF(scaleUp, scaleDown, percentage)
	yOffset := float64(tileSize.Y) * (1 - currentScale.Y)
	xOffset := (float64(tileSize.X) * (1 - currentScale.X)) / 2

	icon := u.getActorIcon(actor, ticksForAnimation)

	flippedX := actor.FlipX() && u.configuration.FlipActorsHorizontally
	if isUnique {
		u.drawUniqueSpecularSheen(actor, float64(screenPos.X)+xOffset, float64(screenPos.Y)+yOffset, atlas, icon, currentScale, drawColor, flippedX)
	} else {
		u.tileRenderer.DrawScaledTileWithFlip(float64(screenPos.X)+xOffset, float64(screenPos.Y)+yOffset, atlas, icon, currentScale, drawColor, flippedX)
	}
}

// actorPhase is a stable pseudo-random tick offset per actor, so bobbing and sheen don't pulse in lockstep.
// Hashes the pointer: heap objects don't move in Go, so it's constant for the actor's lifetime.
func actorPhase(actor *common.Actor) uint64 {
	return uint64(uintptr(unsafe.Pointer(actor))) * 0x9E3779B97F4A7C15 >> 32
}

func (u *UIController) getActorIcon(actor *common.Actor, ticksForAnimation uint64) int32 {
	var icon int32
	switch u.renderingMode {
	case AsciiRendering:
		icon = actor.AsciiIcon(ticksForAnimation)
	case CodePage437Rendering:
		icon = actor.CodePage437Icon(ticksForAnimation)
	case UnicodeRendering:
		icon = actor.UnicodeIcon(ticksForAnimation)
	default:
		icon = actor.Icon(ticksForAnimation)
	}
	return icon
}

func (u *UIController) getTileIcon(tileAt gridmap.Tile) (int32, color.RGBA) {
	tileColor := defs.White
	var icon int32
	switch u.renderingMode {
	case UnicodeRendering:
		fallthrough
	case AsciiRendering:
		icon, tileColor = AsciiIconAndColor(tileAt)
	case CodePage437Rendering:
		icon, tileColor = CodePage437IconAndColor(tileAt)
	default:
		icon = tileAt.Icon()
	}

	return icon, tileColor
}

type RenderingMode int

const (
	GraphicRendering RenderingMode = iota
	AsciiRendering
	CodePage437Rendering
	UnicodeRendering
)

func AsciiIconAndColor(t gridmap.Tile) (int32, color.RGBA) {
	switch t.Special {
	case gridmap.SpecialTileWater:
		return '~', defs.Blue
	case gridmap.SpecialTileForest:
		return ';', defs.Brown
	case gridmap.SpecialTileMountain:
		return '^', defs.LightGray
	case gridmap.SpecialTileVoid:
		return ' ', defs.Black
	case gridmap.SpecialTileStairsUp:
		return '<', defs.White
	case gridmap.SpecialTileStairsDown:
		return '>', defs.White
	case gridmap.SpecialTileDamaging:
		return '~', defs.Red
	}
	if t.IsWalkable {
		return '.', defs.DarkGray
	}
	return '#', defs.OffWhite
}

func CodePage437IconAndColor(t gridmap.Tile) (int32, color.RGBA) {
	switch t.Special {
	case gridmap.SpecialTileWater:
		return int32(util.UnicodeToCP437Byte('≈')), defs.Blue
	case gridmap.SpecialTileForest:
		return int32(util.UnicodeToCP437Byte('♣')), defs.Brown
	case gridmap.SpecialTileMountain:
		return int32(util.UnicodeToCP437Byte('∩')), defs.LightGray
	case gridmap.SpecialTileVoid:
		return int32(util.UnicodeToCP437Byte(' ')), defs.Black
	case gridmap.SpecialTileStairsUp:
		return int32(util.UnicodeToCP437Byte('≡')), defs.White
	case gridmap.SpecialTileStairsDown:
		return int32(util.UnicodeToCP437Byte('≡')), defs.Cyan
	case gridmap.SpecialTileDamaging:
		return int32(util.UnicodeToCP437Byte('≈')), defs.Red
	}

	if t.IsWalkable {
		return int32(util.UnicodeToCP437Byte('∙')), defs.DarkGray
	} else {
		return int32(util.UnicodeToCP437Byte('█')), defs.OffWhite
	}
}
