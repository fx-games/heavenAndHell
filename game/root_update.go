package game

import (
	"errors"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
	"rpg/actions"
	"rpg/common"
	"rpg/geometry"
	"rpg/gridmap"
	"rpg/ui"
)
var ContinueWithAscii = errors.New("ContinueWithAscii")
func (g *GameInstance) Update() error {

	if g.shouldQuitGame {
		return ebiten.Termination
	}

	if g.quitToAscii {
		return ContinueWithAscii
	}

	g.worldTicks++

	player := g.state.GetPlayer()

	if !g.consoleMode {
		g.audioPlayer.Update()

		if g.globalFlashColorTicksLeft > 0 {
			//percentage := float64(g.globalFlashColorTicksLeft) / float64(g.globalFlashColorTicksStart)
			//lerpedColor := util.LerpColor(color.White, g.globalFlashColor, percentage)
			g.tileRenderer.SetGlobalScaleColor(colorAsScaledFloat(g.globalFlashColor, 10.0))
			g.globalFlashColorTicksLeft--
		} else if player.HasStatusEffect(common.StatusBerserk) {
			g.tileRenderer.SetGlobalScaleColor([4]float32{1.0, 0.5, 0.5, 1})
		} else {
			g.tileRenderer.SetGlobalScaleColor([4]float32{1, 1, 1, 1})
		}

		g.handleMouseInput()

		g.handleKeyboardInput()

		g.uiController.Tick()
	}

	g.actionBroker.Tick()


	if !player.IsAlive() {
		return nil
	}
	if !g.consoleMode && !g.setToExactFit && g.configuration.KeepPlayerCentered {
		g.mapRenderer.CenterOnFloat(player.DrawPosition())
	}

	for _, actor := range g.state.GetMap().Actors() {
		actor.Tick()
	}

	return nil
}

func colorAsScaledFloat(lerpedColor color.Color, scale float32) [4]float32 {
	r, g, b, a := lerpedColor.RGBA()
	return [4]float32{float32(r) * scale / 65535.0, float32(g) * scale / 65535.0, float32(b) * scale / 65535.0, float32(a) * scale / 65535.0}
}

func (g *GameInstance) handleKeyboardInput() {
	// must-have
	//  - movement
	//  - wait
	//  - aim ranged attack
	//  - quick ranged attack
	//  - show log
	//  - show inventory
	//  - show stats
	//  - auto explore
	//  - auto attack

	// extra
	//  - pick up item
	//  - quick travel

	g.shiftIsBeingPressedThisTick = ebiten.IsKeyPressed(ebiten.KeyShift)

	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		g.showHelpText()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.uiCancelAll()
		return
	}
	if g.uiController.IsUIModalVisible() {
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			g.uiController.OnCommand(ui.PlayerCommandUp)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			g.uiController.OnCommand(ui.PlayerCommandDown)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			g.uiController.OnCommand(ui.PlayerCommandLeft)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
			g.uiController.OnCommand(ui.PlayerCommandRight)
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.uiController.OnCommand(ui.PlayerCommandConfirm)
			return
		}
		var keys []ebiten.Key
		keys = inpututil.AppendJustPressedKeys(keys)
		for _, key := range keys {
			if key == ebiten.KeyShift || key == ebiten.KeyControl || key == ebiten.KeyAlt {
				continue
			}
			g.uiController.OnKeyPressed(key, g.shiftIsBeingPressedThisTick)
		}
		return
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.PickUpItem()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		g.aimedRangedAttack()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		g.aimedThrow()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		g.quickRangedAttack()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		g.showLog()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.showJournal()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		g.mapRenderer.CenterOnFloat(g.GetWorldState().GetPlayer().DrawPosition())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		g.showFlags()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyPeriod) {
		g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.NewActorCause("Wait", g.state.GetPlayer()), 10))
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		g.openListInventory()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		g.ShowFullStats()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.killLevel()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) {
		g.gotoHellTown()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) {
		g.disableFogOfWar = !g.disableFogOfWar
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		g.showDebugMenu()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		g.isFullScreen = !ebiten.IsFullscreen()
		ebiten.SetFullscreen(g.isFullScreen)
		g.OnScreenSizeChanged()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF12) {
		g.FlashScreen(color.White, 0.1)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		g.recallAtEndOfTurn()
		g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("Travel"), 10))
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyU) {
		level := g.getCurrentDungeonLevel()
		g.transitionTo(gridmap.Transition{
			TargetMap:      levelName(level + 1),
			TargetLocation: "ladder_up",
		})
		g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.AbstractCause("Travel"), 10))
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyO) {
		g.startAutoExplore()

	}
	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.playerAutoAttack()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyJ) {
		g.goOneLevelUp()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyK) {
		g.goOneLevelDown()
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		g.mapRenderer.ScrollBy(geometry.Point{X: 8, Y: 0})
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		g.mapRenderer.ScrollBy(geometry.Point{X: -8, Y: 0})
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		g.mapRenderer.ScrollBy(geometry.Point{X: 0, Y: -8})
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		g.mapRenderer.ScrollBy(geometry.Point{X: 0, Y: 8})
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyW) {
		g.ManualMovePlayer(geometry.Point{X: 0, Y: -1})
	} else if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		g.ManualMovePlayer(geometry.Point{X: 0, Y: 1})
	} else if inpututil.IsKeyJustPressed(ebiten.KeyA) {
		g.ManualMovePlayer(geometry.Point{X: -1, Y: 0})
	} else if inpututil.IsKeyJustPressed(ebiten.KeyD) {
		g.ManualMovePlayer(geometry.Point{X: 1, Y: 0})
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.SetMapZoom(1)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		g.SetMapZoom(-1)
	}
}

func (g *GameInstance) uiCancelAll() {
	if g.targeter != nil {
		g.CancelTargeting()
		return
	}
	if g.uiController.IsUIModalVisible() {
		g.uiController.OnCommand(ui.PlayerCommandCancel)
		return
	}
	if g.inventory.IsActive() {
		g.inventory.Deactivate()
		return
	}
	g.cancelAutoMove()
}

func (g *GameInstance) startAutoExplore() {
	if !g.state.GetPlayer().CanAutoMove() {
		g.Log("You can't explore right now")
		return
	}
	target, hasTarget := g.getNextAutoExploreTarget()
	if g.autoExplore {
		g.stopAutoMove()
		g.Log("You stop exploring")
		return
	}
	if hasTarget {
		g.Log("You start exploring")
		g.autoExplore = true
		g.startPlayerAutoMovement(target)
	} else {
		g.Log("Nothing to explore")
	}
}

func (g *GameInstance) goOneLevelUp() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	currentMap := g.state.GetMap()
	mapName := currentMap.GetName()
	if mapName == string(LevelNameTown) {
		return
	}

	ourPos := g.state.GetPlayer().MapPosition()
	nearestStairsUp := currentMap.GetNearestTile(ourPos, func(tile gridmap.Tile) bool {
		return tile.IsStairsUp()
	})

	if nearestStairsUp == ourPos {
		g.Log("No known path")
		return
	}
	g.cancelAutoMove()
	g.startPlayerAutoMovement(nearestStairsUp)
}

func (g *GameInstance) goOneLevelDown() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	currentMap := g.state.GetMap()
	//mapName := currentMap.Action()

	ourPos := g.state.GetPlayer().MapPosition()
	nearestStairsDown := currentMap.GetNearestTile(ourPos, func(tile gridmap.Tile) bool {
		return tile.IsStairsDown()
	})
	if nearestStairsDown == ourPos {
		g.Log("No known path")
		return
	}
	g.cancelAutoMove()
	g.startPlayerAutoMovement(nearestStairsDown)
}

func (g *GameInstance) quickRangedAttack() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	if !g.state.GetPlayer().GetEquipment().HasRangedWeaponEquipped() {
		g.Log("You don't have a ranged weapon equipped")
		return
	}
	if g.targeter != nil {
		g.targeter.OnCommand(ui.PlayerCommandConfirm)
	} else {
		g.BeginRangedTargeting(g.playerRangedAttack)
		if g.targeter.HasValidTarget() {
			g.targeter.OnCommand(ui.PlayerCommandConfirm)
		} else {
			g.targeter = nil
		}
	}
	return
}

func (g *GameInstance) aimedRangedAttack() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	if !g.state.GetPlayer().GetEquipment().HasRangedWeaponEquipped() {
		g.Log("You don't have a ranged weapon equipped")
		return
	}
	if g.targeter != nil {
		g.targeter.OnCommand(ui.PlayerCommandConfirm)
	} else {
		g.BeginRangedTargeting(g.playerRangedAttack) // execute ranged attack action and end turn)
	}
	return
}

func (g *GameInstance) aimedThrow() {
	if !g.state.GetPlayer().IsAlive() {
		return
	}
	if g.targeter != nil {
		g.targeter.OnCommand(ui.PlayerCommandConfirm)
		return
	}
	equipment := g.state.GetPlayer().GetEquipment()
	inventory := g.state.GetPlayer().GetInventory()
	if !equipment.HasThrowableQuivered() {
		throwFilter := func(element common.Item) bool {
			return element.IsThrowable()
		}
		onSelected := func(itemKey rune) {
			g.uiController.PopAll()
			thrownItem := inventory.GetItemByLetter(itemKey)
			throwComponent := thrownItem.GetThrowStats()
			g.BeginThrowTargeting(throwComponent.HitZone, g.playerThrow(thrownItem))
		}
		g.openListInventoryForSelection(throwFilter, onSelected)
	} else {
		thrownItem := equipment.GetNextQuiveredThrowable()
		throwComponent := thrownItem.GetThrowStats()
		g.BeginThrowTargeting(throwComponent.HitZone, g.playerThrow(thrownItem))
	}
}

func (g *GameInstance) handleMouseInput() {
	mousePosInPixelsX, mousePosInPixelsY := ebiten.CursorPosition()
	mousePosInPixelsX = int(float64(mousePosInPixelsX) / g.deviceDPIScale)
	mousePosInPixelsY = int(float64(mousePosInPixelsY) / g.deviceDPIScale)

	mouseMoved := false
	if g.mousePosInPixels.X != mousePosInPixelsX || g.mousePosInPixels.Y != mousePosInPixelsY {
		mouseMoved = true
	}
	g.mousePosInPixels.X = mousePosInPixelsX
	g.mousePosInPixels.Y = mousePosInPixelsY

	newMapPos := g.mapRenderer.GetMapCellAtScreenPos(g.mousePosInPixels)
	/*
	   if newMapPos != g.mousePosOnMap {
	       println(fmt.Sprintf("Mouse pos: %d, %d", newMapPos.X, newMapPos.Y))
	   }

	*/
	if newMapPos != g.mousePosOnMap && g.targeter != nil {
		g.targeter.OnMouseMoved(newMapPos)
	}
	g.mousePosOnMap = newMapPos

	if mouseMoved {
		handled, toolTip := g.uiController.OnMouseMoved(mousePosInPixelsX, mousePosInPixelsY)
		if handled {
			g.uiController.ShowTooltipWithDelay(toolTip)
		} else {
			g.uiController.HideTooltip()
		}

	}
	noButton := ebiten.MouseButton(-1)
	mouseButton := noButton
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mouseButton = ebiten.MouseButtonLeft
	} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		mouseButton = ebiten.MouseButtonRight
	}
	if mouseButton != noButton {
		handledByUI := g.uiController.OnMouseClicked(mouseButton, mousePosInPixelsX, mousePosInPixelsY)
		if handledByUI {
			return
		}
		visibleMap := g.mapRenderer.GetVisibleMap()
		if !g.state.GetMap().Contains(g.mousePosOnMap) || !visibleMap.Contains(g.mousePosOnMap) {
			return
		}
		if g.targeter != nil {
			g.targeter.OnMouseClicked(g.mousePosOnMap)
			return
		}
		if !handledByUI {
			g.onMapClicked(mouseButton, g.mousePosOnMap)
		}
	}
}
