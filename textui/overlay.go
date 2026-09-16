package textui

import (
	"image/color"

	"github.com/memmaker/go/geometry"
	"github.com/memmaker/go/textiles"
)

// Overlay holds text labels drawn over map positions, e.g. a target's hit chance.
type Overlay struct {
	grid              map[geometry.Point]textiles.TextIcon
	defaultBackground color.RGBA
	defaultForeground color.RGBA
}

func NewOverlay() *Overlay {
	return &Overlay{grid: make(map[geometry.Point]textiles.TextIcon)}
}

func (o *Overlay) SetDefaultColors(fg, bg color.RGBA) {
	o.defaultForeground = fg
	o.defaultBackground = bg
}

func (o *Overlay) Set(x, y int, icon textiles.TextIcon) {
	o.grid[geometry.Point{X: x, Y: y}] = icon
}

func (o *Overlay) ClearAll() {
	clear(o.grid)
}

func (o *Overlay) Print(x, y int, text string) {
	o.PrintColored(x, y, text, o.defaultForeground, o.defaultBackground)
}

func (o *Overlay) PrintColored(x, y int, text string, fg, bg color.RGBA) {
	for i, r := range []rune(text) {
		o.Set(x+i, y, textiles.TextIcon{Char: r, Fg: fg, Bg: bg})
	}
}

func (o *Overlay) IsSet(x, y int) bool {
	_, ok := o.grid[geometry.Point{X: x, Y: y}]
	return ok
}

func (o *Overlay) Get(x, y int) textiles.TextIcon {
	return o.grid[geometry.Point{X: x, Y: y}]
}

func (o *Overlay) AddAbove(pos geometry.Point, text string) {
	xOffset := len(text) / 2
	labelPos := pos.Add(geometry.Point{X: -xOffset, Y: -1})
	o.Print(labelPos.X, labelPos.Y, text)
}

func (o *Overlay) AddBelow(pos geometry.Point, text string) {
	xOffset := len(text) / 2
	labelPos := pos.Add(geometry.Point{X: -xOffset, Y: 1})
	o.Print(labelPos.X, labelPos.Y, text)
}
