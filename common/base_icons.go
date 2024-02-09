package common

import "image/color"

type IconIndex int
type TextIcon struct {
	Rune   rune
	Fg, Bg color.RGBA
}
type BitmapIcon struct {
	AtlasID   int
	TileIndex int32
	TintColor color.RGBA
}
