package gui

import (
	"image/color"
	"rpg/util"
)

// TODO: DO we need this? should only need the tilerenderer??
type UIElement interface {
	Name() string
	GetTooltipLines(textParser func(string) []util.ColoredTextPart) []util.LabelText
	InventoryIcon() int32
	TintColor() color.RGBA
	Counter() int
	String() string
}
