package util

import (
	"image/color"
)

type MenuItem struct {
	Text               string
	TextWithColorCodes string
	UseColorCodes      bool
	Action             func()
	TextColor          color.Color
	CharIcon           int32
	TooltipText        []LabelText
	ActionLeft         func() string
	ActionRight        func() string
}

func (m MenuItem) String() string {
	return m.Text
}

type LabelText struct {
	Text               string
	TextColor          color.Color
	UseColorCodes      bool
	TextWithColorCodes string
}

func (t LabelText) String() string {
	return t.Text
}

func (t LabelText) WithColor(textColor color.Color) LabelText {
	t.TextColor = textColor
	return t
}

func IndexToXY(index int, width int) (int, int) {
	return index % width, index / width
}

func XYToIndex(x int, y int, width int) int {
	return y*width + x
}
