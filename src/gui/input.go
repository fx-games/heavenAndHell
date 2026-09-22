package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"rpg/src/common"
	"rpg/src/geometry"
	"rpg/src/util"
)

type InputReceiver interface {
	OnCommand(command CommandType) bool
	OnMouseClicked(button ebiten.MouseButton, x int, y int) bool
	OnMouseMoved(x int, y int) (bool, Tooltip)
	OnMouseWheel(x int, y int, dy float64) bool
}

type TextInputReceiver interface {
	InputReceiver
	OnKeyPressed(key ebiten.Key, shiftPressed bool)
	SetTick(tick uint64)
}
type MouseEvent int

const (
	MouseEventLeftClicked MouseEvent = iota
	MouseEventMoved
)

type CommandType int

const (
	PlayerCommandUp CommandType = iota
	PlayerCommandDown
	PlayerCommandLeft
	PlayerCommandRight

	PlayerCommandUpLeft
	PlayerCommandUpRight
	PlayerCommandDownLeft
	PlayerCommandDownRight
	PlayerCommandConfirm
	PlayerCommandCancel
	PlayerCommandOptions
)

func isBeingPressedRepeatedly(key ebiten.Key) bool {
	pressedForTicks := inpututil.KeyPressDuration(key)
	ticksBetweenMovement := 20
	if pressedForTicks > 30 {
		ticksBetweenMovement = 8
	}
	isRepetitionTick := pressedForTicks > 0 && pressedForTicks%ticksBetweenMovement == 0
	return inpututil.IsKeyJustPressed(key) || (ebiten.IsKeyPressed(key) && isRepetitionTick)
}

func (u *UIController) handleKeyboardInput() {
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

	act := u.context.PlayerAction()

	u.shiftIsBeingPressedThisTick = ebiten.IsKeyPressed(ebiten.KeyShift)

	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		u.showHelpText()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if u.canCancel() {
			u.Cancel()
		} else {
			u.openPauseMenu()
		}
		return
	}
	if u.IsUIModalVisible() {
		if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			u.OnCommand(PlayerCommandUp)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			u.OnCommand(PlayerCommandDown)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			u.OnCommand(PlayerCommandLeft)
			return
		} else if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
			u.OnCommand(PlayerCommandRight)
			return
		}
		if _, sheet := u.GetTopModal().(*CharacterSheet); sheet && inpututil.IsKeyJustPressed(ebiten.KeyC) {
			u.PopWidget() // 'c' toggles it shut, same key that opened it
			return
		}
		switch u.GetTopModal().(type) {
		case Keylistener: // letters are hotkeys or typed text there
		case *BasicLabel, *InfoWindow: // pop-ups: e and space close them like esc, enter and any click
			if inpututil.IsKeyJustPressed(ebiten.KeyE) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
				u.Cancel()
				return
			}
		default:
			if inpututil.IsKeyJustPressed(ebiten.KeyE) {
				u.OnCommand(PlayerCommandConfirm)
				return
			}
			// WASD mirror the arrows; unhandled ones still reach letter shortcuts below
			wasd := map[ebiten.Key]CommandType{ebiten.KeyW: PlayerCommandUp, ebiten.KeyS: PlayerCommandDown, ebiten.KeyA: PlayerCommandLeft, ebiten.KeyD: PlayerCommandRight}
			for key, command := range wasd {
				if inpututil.IsKeyJustPressed(key) && u.OnCommand(command) {
					return
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			u.OnCommand(PlayerCommandConfirm)
			return
		}
		var keys []ebiten.Key
		keys = inpututil.AppendJustPressedKeys(keys)
		for _, key := range keys {
			if key == ebiten.KeyShift || key == ebiten.KeyControl || key == ebiten.KeyAlt {
				continue
			}
			u.OnKeyPressed(key, u.shiftIsBeingPressedThisTick)
		}
		return
	}

	if t := u.targeter; t != nil && t.hasConfirm && inpututil.IsKeyJustPressed(t.confirmKey) {
		t.Activate()
		u.CancelTargeting()
		return
	}
	if t := u.targeter; t != nil { // aiming: Tab cycles targets, WASD nudges the cursor, Enter fires
		if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
			t.Cycle(map[bool]int{false: 1, true: -1}[u.shiftIsBeingPressedThisTick])
			return
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			t.Activate()
			u.CancelTargeting()
			return
		}
		for key, command := range map[ebiten.Key]CommandType{ebiten.KeyW: PlayerCommandUp, ebiten.KeyS: PlayerCommandDown, ebiten.KeyA: PlayerCommandLeft, ebiten.KeyD: PlayerCommandRight} {
			if isBeingPressedRepeatedly(key) {
				t.OnCommand(command)
				return
			}
		}
	}
	for i := range common.BeltSize {
		if isBeingPressedRepeatedly(ebiten.Key1 + ebiten.Key(i)) {
			u.UseBeltSlot(i)
			return
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		act.PickUpItem()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF) {
		if u.targeter != nil && u.targeter.HasValidTarget() {
			u.targeter.Activate()
			u.CancelTargeting()
		} else {
			act.AimedRangedAttack()
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyE) {
		u.openSpellMenu()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		act.QuickRangedAttack()
	}
	if isBeingPressedRepeatedly(ebiten.KeySpace) {
		act.CastLastSpellAtNearest()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		u.ShowLog()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		u.OpenTextWindow(util.ToLabelText(u.context.GameState().D1QuestLog()))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyZ) {
		act.RestUntilHealed()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		u.mapRenderer.CenterOnFloat(u.context.GameState().GetPlayer().DrawPosition())
		u.manualScroll = false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		u.showFlags()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyPeriod) {
		act.EndTurn(10)
		//g.actionBroker.Execute(actions.NewEndPlayerTurnAction(actions.NewActorCause("Wait", g.state.GetPlayer()), 10))
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyI) {
		u.OpenListInventory()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		u.ToggleFullStats()
	}
	// F5 / F9 quick save and quick load; the debug views they used to share the function keys with moved
	// into the debug menu (F10).
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) && u.context.QuickSave != nil {
		u.context.QuickSave()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF9) && u.context.QuickLoad != nil {
		u.context.QuickLoad()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		u.context.DebugActions().OpenDebugMenu()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		u.toggleFullscreen()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyT) {
		act.AimedThrow()
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyU) {
		u.openLevelUpMenu() // spend stat points
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyO) {
		act.StartAutoExplore()
	}

	if isBeingPressedRepeatedly(ebiten.KeyTab) {
		act.AutoAttack()
	}

	for key, direction := range map[ebiten.Key]geometry.Point{
		ebiten.KeyArrowRight: {X: 8, Y: 0},
		ebiten.KeyArrowLeft:  {X: -8, Y: 0},
		ebiten.KeyArrowUp:    {X: 0, Y: -8},
		ebiten.KeyArrowDown:  {X: 0, Y: 8},
	} {
		if ebiten.IsKeyPressed(key) {
			u.mapRenderer.ScrollBy(direction)
			u.manualScroll = true
		}
	}

	if isBeingPressedRepeatedly(ebiten.KeyW) {
		act.ManualMovePlayer(geometry.Point{X: 0, Y: -1})
	} else if isBeingPressedRepeatedly(ebiten.KeyS) {
		act.ManualMovePlayer(geometry.Point{X: 0, Y: 1})
	} else if isBeingPressedRepeatedly(ebiten.KeyA) {
		act.ManualMovePlayer(geometry.Point{X: -1, Y: 0})
	} else if isBeingPressedRepeatedly(ebiten.KeyD) {
		act.ManualMovePlayer(geometry.Point{X: 1, Y: 0})
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		u.ChangeMapZoom(1)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		u.ChangeMapZoom(-1)
	}
}

func (u *UIController) handleMouseInput() {
	mousePosInPixelsX, mousePosInPixelsY := ebiten.CursorPosition()
	mousePosInPixelsX = int(float64(mousePosInPixelsX) / u.deviceDPIScale)
	mousePosInPixelsY = int(float64(mousePosInPixelsY) / u.deviceDPIScale)

	mouseMoved := false
	if u.mousePosInPixels.X != mousePosInPixelsX || u.mousePosInPixels.Y != mousePosInPixelsY {
		mouseMoved = true
	}
	u.mousePosInPixels.X = mousePosInPixelsX
	u.mousePosInPixels.Y = mousePosInPixelsY

	newMapPos := u.mapRenderer.GetMapCellAtScreenPos(u.mousePosInPixels)
	/*
	   if newMapPos != u.mousePosOnMap {
	       println(fmt.Sprintf("Mouse pos: %d, %d", newMapPos.X, newMapPos.Y))
	   }

	*/
	if newMapPos != u.mousePosOnMap && u.targeter != nil {
		u.targeter.OnMouseMoved(newMapPos)
	}
	u.mousePosOnMap = newMapPos

	if mouseMoved {
		handled, toolTip := u.OnMouseMoved(mousePosInPixelsX, mousePosInPixelsY)
		if handled {
			u.ShowTooltipWithDelay(toolTip)
		} else {
			u.HideTooltip()
			u.updateMapTileHoverInfo(newMapPos)
		}

	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		u.endIconDrag(mousePosInPixelsX, mousePosInPixelsY)
	}
	noButton := ebiten.MouseButton(-1)
	mouseButton := noButton
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mouseButton = ebiten.MouseButtonLeft
	} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		mouseButton = ebiten.MouseButtonRight
	}
	if mouseButton != noButton {
		handledByUI := u.OnMouseClicked(mouseButton, mousePosInPixelsX, mousePosInPixelsY)
		if handledByUI {
			return
		}
		visibleMap := u.mapRenderer.GetVisibleMap()
		state := u.context.GameState()
		if !state.GetMap().Contains(u.mousePosOnMap) || !visibleMap.Contains(u.mousePosOnMap) {
			return
		}
		if t := u.targeter; t != nil { // the hit effect may clear or replace u.targeter itself
			t.OnMouseClicked(u.mousePosOnMap)
			if t.IsDone() && u.targeter == t {
				u.CancelTargeting()
			}
			return
		}
		if !handledByUI {
			u.onMapClicked(mouseButton, u.mousePosOnMap)
		}
	}
}
func (u *UIController) onMapClicked(mouseButton ebiten.MouseButton, mapPos geometry.Point) {
	if u.IsUIModalVisible() {
		return
	}
	act := u.context.PlayerAction()
	state := u.context.GameState()
	currentMap := state.GetMap()
	if mouseButton == ebiten.MouseButtonRight && currentMap.IsActorAt(mapPos) && currentMap.ActorAt(mapPos).IsVisible() && state.GetPlayer().CanPerceive(mapPos) {
		act.ShowActorInfoAt(mapPos)
	} else if mouseButton == ebiten.MouseButtonRight && currentMap.IsObjectAt(mapPos) && state.GetPlayer().CanSee(mapPos) {
		act.ShowObjectInfoAt(mapPos)
	} else if mouseButton == ebiten.MouseButtonLeft && act.CloseDoorAt(mapPos) {
		// clicked an open door next to the player: it shuts
	} else if mouseButton == ebiten.MouseButtonLeft && currentMap.IsExplored(mapPos) && act.RangedAttackAt(mapPos) {
		// fired at the hostile actor under the cursor
	} else if mouseButton == ebiten.MouseButtonLeft && currentMap.IsExplored(mapPos) {
		act.CancelAutoMove()
		act.StartPlayerAutoMovement(mapPos)
	}
}
