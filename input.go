package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"rpg/actions"
	"rpg/geometry"
	"rpg/ui"
	"rpg/util"
)

type InputCommand string

const (
	CommandMove             InputCommand = "move"
	CommandMeleeAttack      InputCommand = "melee_attack"
	CommandRangedAttack     InputCommand = "ranged_attack"
	CommandTargetedAction   InputCommand = "targeted_action"
	CommandUntargetedAction InputCommand = "untargeted_action"
)

type BaseCommand struct {
	commandType InputCommand
}

func (d BaseCommand) CommandType() InputCommand {
	return d.commandType
}

type DirectionalCommand struct {
	BaseCommand
	direction geometry.CompassDirection
}

type TargetedCommand struct {
	BaseCommand
	targetPos geometry.Point
}

type Command interface {
	CommandType() InputCommand
}

func getPlayerAction(gameCommand Command, state actions.WorldState) actions.Action {
	// possible actions
	// untargeted_action()
	// movement(direction)
	// targeted_action(target_pos)

	return nil
}

func handleKeyboardInput(actionReceiver ui.InputReceiver) bool {
	if util.IsBeingPressedRepeatedly(ebiten.KeyArrowRight) || util.IsBeingPressedRepeatedly(ebiten.KeyNumpad6) {
		return actionReceiver.OnCommand(ui.PlayerCommandRight)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyArrowLeft) || util.IsBeingPressedRepeatedly(ebiten.KeyNumpad4) {
		return actionReceiver.OnCommand(ui.PlayerCommandLeft)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyArrowUp) || util.IsBeingPressedRepeatedly(ebiten.KeyNumpad8) {
		return actionReceiver.OnCommand(ui.PlayerCommandUp)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyArrowDown) || util.IsBeingPressedRepeatedly(ebiten.KeyNumpad2) {
		return actionReceiver.OnCommand(ui.PlayerCommandDown)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyNumpad9) {
		return actionReceiver.OnCommand(ui.PlayerCommandUpRight)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyNumpad7) {
		return actionReceiver.OnCommand(ui.PlayerCommandUpLeft)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyNumpad3) {
		return actionReceiver.OnCommand(ui.PlayerCommandDownRight)
	}
	if util.IsBeingPressedRepeatedly(ebiten.KeyNumpad1) {
		return actionReceiver.OnCommand(ui.PlayerCommandDownLeft)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return actionReceiver.OnCommand(ui.PlayerCommandConfirm)
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		return actionReceiver.OnCommand(ui.PlayerCommandCancel)
	}

	return false
}
