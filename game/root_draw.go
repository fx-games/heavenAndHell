package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/common"
	"rpg/geometry"
	"rpg/util"
)

func (g *GameInstance) Draw(screen *ebiten.Image) {
	g.tileRenderer.SetRenderTarget(screen)

	g.mapRenderer.Draw(g.worldTicks)

	g.drawActors()

	g.actionBroker.Draw(g.hybridRenderer)

	if !g.uiController.IsMouseCapturedByUI() {
		if g.targeter != nil {
			g.targeter.Draw()
		} else {
			// default map selection rectangle
			g.hybridRenderer.DrawOnMap(g.mousePosOnMap, common.IconTargeting, color.White)
		}
	}

	g.uiController.Draw()

	if g.message.ticksLeft > 0 {
		_, height := g.tileRenderer.MeasureString(g.message.GetText())
		//g.tileRenderer.DrawTTFOnScreen(1, height+2, common.RemoveColorCodes(g.message.GetText()), color.Black)
		g.tileRenderer.DrawTTFOnScreenWithColorCodes(1, height, g.message.GetText())
		g.message.ticksLeft--
	}

	for _, debugLine := range g.debugLines {
		g.tileRenderer.DebugLine(debugLine.Item1, debugLine.Item2)
	}

	for _, debugCellPos := range g.debugCells {
		g.hybridRenderer.DrawOnMap(debugCellPos, common.IconTargeting, color.White)
	}
}

func (g *GameInstance) drawActors() {
	visibleMapOnScreen := g.mapRenderer.GetVisibleMap()
	playerFieldOfView := g.state.GetPlayer().GetFOV()

	for _, downedActor := range g.state.GetMap().DownedActors() {
		if !visibleMapOnScreen.Contains(downedActor.MapPosition()) || !playerFieldOfView.Visible(downedActor.MapPosition()) {
			continue
		}
		if !g.configuration.DrawTilesBehindActors && g.state.GetMap().IsActorAt(downedActor.MapPosition()) {
			continue
		}
		screenPos := g.mapRenderer.MapFloatToScreen(downedActor.DrawPosition())
		g.tileRenderer.DrawDefaultScaleTile(float64(screenPos.X), float64(screenPos.Y), g.worldTexture, downedActor.Icon(g.worldTicks), g.applyLighting(downedActor.MapPosition(), common.White))
	}

	for _, actor := range g.state.GetMap().Actors() {
		if !visibleMapOnScreen.Contains(actor.MapPosition()) || !playerFieldOfView.Visible(actor.MapPosition()) {
			continue
		}
		var drawColor color.Color
		drawColor = g.applyLighting(actor.MapPosition(), common.White)

		if actor.IsHurt() {
			percentage := actor.GetHurtTicksAsPercentage() // goes from 1.0 to 0.0
			drawColor = util.LerpColor(common.Red, color.White, percentage)
		}
		if g.configuration.ActorBobbingEnabled {
			g.drawActorWithBobbingAnimation(actor, drawColor)
		} else {
			screenPos := g.mapRenderer.MapFloatToScreen(actor.DrawPosition())
			g.tileRenderer.DrawDefaultScaleTileWithFlip(float64(screenPos.X), float64(screenPos.Y), g.worldTexture, actor.Icon(g.worldTicks), drawColor, actor.FlipX())
		}
	}
}

func (g *GameInstance) drawActorWithBobbingAnimation(actor *common.Actor, drawColor color.Color) {
	screenPos := g.mapRenderer.MapFloatToScreen(actor.DrawPosition())
	actorTickOffset := actor.MapPosition().X + actor.MapPosition().Y + len(actor.Name())
	tileSize := g.mapRenderer.GetScaledTileSize()
	ticksForAnimation := g.worldTicks + uint64(actorTickOffset)
	percentage := util.GetPercentageFromTick(ticksForAnimation, 1) // -> [0.0..1.0)  but we want [0.0..1.0..0.0]
	percentage *= 2
	if percentage > 1 {
		percentage = 1 - percentage
	}
	//g.tileRenderer.DrawDefaultScaleTile(float64(screenPos.X), float64(screenPos.Y), g.worldTexture, actor.Icon(g.worldTicks), color.White)
	scaleUp := geometry.PointF{X: 1, Y: 1}
	scaleDown := geometry.PointF{X: 1.05, Y: 0.95}
	currentScale := util.LerpPointF(scaleUp, scaleDown, percentage)
	yOffset := float64(tileSize.Y) * (1 - currentScale.Y)
	xOffset := (float64(tileSize.X) * (1 - currentScale.X)) / 2
	g.tileRenderer.DrawScaledTileWithFlip(float64(screenPos.X)+xOffset, float64(screenPos.Y)+yOffset, g.worldTexture, actor.Icon(ticksForAnimation), currentScale, drawColor, actor.FlipX())
}
