package ui

import "github.com/hajimehoshi/ebiten/v2"

type InputReceiver interface {
	OnCommand(command CommandType) bool
	OnMouseClicked(button ebiten.MouseButton, x int, y int) bool
	OnMouseMoved(x int, y int) (bool, Tooltip)
	OnMouseWheel(x int, y int, dy float64) bool
}

type TextInputReceiver interface {
	InputReceiver
	OnKeyPressed(key ebiten.Key)
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
