package util

import "image/color"

type ColoredTextPart struct {
	Text    string
	Color   color.RGBA
	XOffset float64
	Height  float64
}
