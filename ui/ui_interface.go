package ui

import (
	"image/color"
	"rpg/util"
)

// TODO: DO we need this? should only need the tilerenderer??
type UIElement interface {
	Name() string
	GetTooltipLines() []util.LabelText
	InventoryIcon() int32
	TintColor() color.Color
	Counter() int
	String() string
}
