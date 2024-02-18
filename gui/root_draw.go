package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/stats"
	"rpg/util"
)

func (u *UIController) Draw(screen *ebiten.Image) {
	u.tileRenderer.SetRenderTarget(screen)

	u.mapRenderer.Draw(u.worldTicks)

	u.drawActors()

	u.context.Animator().Draw(u.mapRenderer) // should be some "animator"

	if !u.IsMouseCapturedByUI() {
		if u.targeter != nil {
			u.targeter.Draw(u.mapRenderer)
		} else {
			// default map selection rectangle
			u.mapRenderer.DrawOnMap(u.mousePosOnMap, defs.IconTargeting, color.White)
		}
	}

	u.drawUI()

	if u.message.ticksLeft > 0 {

		//u.tileRenderer.DrawTTFOnScreen(1, height+2, common.RemoveColorCodes(u.message.GetText()), color.Black)
		u.tileRenderer.DrawTTFOnScreenWithColorCodes(1, u.message.GetHeight(), u.message.GetColorcodedText())
		u.message.ticksLeft--
	}

	if u.currentTooltip != nil && !u.currentTooltip.IsNull() && u.ticksUntilTooltipAppears == 0 {
		u.currentTooltip.Draw()
	}
}

func (u *UIController) drawActors() {
	visibleMapOnScreen := u.mapRenderer.GetVisibleMap()
	state := u.context.GameState()
	playerFieldOfView := state.GetPlayer().GetFOV()

	for _, downedActor := range state.GetMap().DownedActors() {
		if !visibleMapOnScreen.Contains(downedActor.MapPosition()) || !playerFieldOfView.Visible(downedActor.MapPosition()) {
			continue
		}
		if !u.configuration.DrawTilesBehindActors && state.GetMap().IsActorAt(downedActor.MapPosition()) {
			continue
		}
		screenPos := u.mapRenderer.MapFloatToScreen(downedActor.DrawPosition())
		u.tileRenderer.DrawDefaultScaleTile(float64(screenPos.X), float64(screenPos.Y), u.worldTexture, downedActor.Icon(u.worldTicks), u.context.GameState().ApplyLighting(downedActor.MapPosition(), defs.White, u.worldTicks))
	}

	for _, actor := range state.GetMap().Actors() {
		if !visibleMapOnScreen.Contains(actor.MapPosition()) || !playerFieldOfView.Visible(actor.MapPosition()) {
			continue
		}
		if !actor.IsVisible() {
			continue
		}
		var drawColor color.Color
		drawColor = state.ApplyLighting(actor.MapPosition(), defs.White, u.worldTicks)

		if actor.IsHurt() {
			percentage := actor.GetHurtTicksAsPercentage() // goes from 1.0 to 0.0
			drawColor = util.LerpColor(defs.Red, color.White, percentage)
		}
		if u.configuration.ActorBobbingEnabled {
			u.drawActorWithBobbingAnimation(actor, drawColor)
		} else {
			screenPos := u.mapRenderer.MapFloatToScreen(actor.DrawPosition())
			u.tileRenderer.DrawDefaultScaleTileWithFlip(float64(screenPos.X), float64(screenPos.Y), u.worldTexture, actor.Icon(u.worldTicks), drawColor, actor.FlipX())
		}
	}

	for _, actor := range state.GetMap().Actors() {
		if !visibleMapOnScreen.Contains(actor.MapPosition()) || !playerFieldOfView.Visible(actor.MapPosition()) {
			continue
		}
		if !actor.GetStats().GetResource(stats.Health).IsAtMax() {
			u.drawHealthBar(actor)
		}
	}
}

func (u *UIController) drawActorWithBobbingAnimation(actor *common.Actor, drawColor color.Color) {
	screenPos := u.mapRenderer.MapFloatToScreen(actor.DrawPosition())
	actorTickOffset := actor.MapPosition().X + actor.MapPosition().Y + len(actor.Name())
	tileSize := u.mapRenderer.GetScaledTileSize()
	ticksForAnimation := u.worldTicks + uint64(actorTickOffset)
	percentage := util.GetPercentageFromTick(ticksForAnimation, 1) // -> [0.0..1.0)  but we want [0.0..1.0..0.0]
	percentage *= 2
	if percentage > 1 {
		percentage = 1 - percentage
	}
	//u.tileRenderer.DrawDefaultScaleTile(float64(screenPos.X), float64(screenPos.Y), u.worldTexture, actor.Icon(u.worldTicks), color.White)
	scaleUp := geometry.PointF{X: 1, Y: 1}
	scaleDown := geometry.PointF{X: 1.05, Y: 0.95}
	currentScale := util.LerpPointF(scaleUp, scaleDown, percentage)
	yOffset := float64(tileSize.Y) * (1 - currentScale.Y)
	xOffset := (float64(tileSize.X) * (1 - currentScale.X)) / 2
	u.tileRenderer.DrawScaledTileWithFlip(float64(screenPos.X)+xOffset, float64(screenPos.Y)+yOffset, u.worldTexture, actor.Icon(ticksForAnimation), currentScale, drawColor, actor.FlipX())
}
