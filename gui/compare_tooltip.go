package gui

import (
	"rpg/common"
	"rpg/defs"
	"rpg/geometry"
	"rpg/util"
)

// CompareTooltip shows an item's tooltip on the right and the equipped items it would replace stacked on the left
// under an "Equipped" header, all kept on screen.
type CompareTooltip struct {
	header *BasicLabel
	left   []*BasicLabel
	item   *BasicLabel
}

const compareTooltipGap = 10

// NewElementTooltip is an inventory element's tooltip, compared to the equipped items if it is an unequipped item.
func NewElementTooltip(u *UIController, element interface {
	GetTooltipLines(func(string) []util.ColoredTextPart) []util.LabelText
}, mousePos geometry.Point) Tooltip {
	if item := firstItemFromUIStack(element); item != nil {
		return NewItemTooltip(u, item, mousePos)
	}
	return NewTextTooltip(u, element.GetTooltipLines(u.ParseColorCodedText), mousePos)
}

// NewItemTooltip is an item's tooltip, compared to the gear it would replace. An item already worn has nothing to
// compare against, so it gets the plain tooltip.
func NewItemTooltip(u *UIController, item *common.Item, mousePos geometry.Point) Tooltip {
	if u.context.GameState().GetPlayer().GetEquipment().IsEquipped(item) {
		return NewTextTooltip(u, item.GetTooltipLines(u.ParseColorCodedText), mousePos)
	}
	return NewItemCompareTooltip(u, item, mousePos)
}

// NewItemCompareTooltip returns the plain tooltip if equipping item displaces nothing.
func NewItemCompareTooltip(u *UIController, item *common.Item, mousePos geometry.Point) Tooltip {
	equipped, itemCmp := u.context.GameState().GetPlayer().GetEquipment().D1CompareForEquipping(item)
	if len(equipped) == 0 {
		return NewTextTooltip(u, item.GetTooltipLines(u.ParseColorCodedText), mousePos)
	}
	t := &CompareTooltip{
		header: NewTextTooltip(u, []util.LabelText{util.LabelTextFromStringWithColor("Equipped", defs.OffWhite)}, mousePos),
		item:   NewTextTooltip(u, item.D1TooltipLinesComparedTo(itemCmp, u.ParseColorCodedText), mousePos),
	}
	for _, c := range equipped {
		lines := c.Item.D1TooltipLinesComparedTo(c.Cmp, u.ParseColorCodedText)
		switch c.Note {
		case "":
		case common.NoteUnequipped:
			lines = append(lines, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(c.Note, defs.NegativeHighlightColor))
		default:
			lines = append(lines, util.LabelTextFromString(""), util.LabelTextFromStringWithColor(c.Note, defs.LightGray))
		}
		t.left = append(t.left, NewTextTooltip(u, lines, mousePos))
	}
	t.layout(u, mousePos)
	return t
}

// outer is a label's drawn size including its border (see BasicLabel.Draw).
func outer(l *BasicLabel) geometry.Point {
	return l.posInfo.SizeInPixels.Add(l.borderPadding).Add(geometry.Point{X: 4, Y: 4})
}

// place moves a label so its border's top left is at p.
func place(l *BasicLabel, p geometry.Point) {
	l.topLeft = p.Add(l.borderPadding.Div(2))
}

func (t *CompareTooltip) layout(u *UIController, mousePos geometry.Point) {
	// the left column shares one width
	column := append([]*BasicLabel{t.header}, t.left...)
	colWidth := 0
	for _, l := range column {
		colWidth = max(colWidth, l.posInfo.SizeInPixels.X)
	}
	leftHeight := -compareTooltipGap
	for _, l := range column {
		l.posInfo.SizeInPixels.X = colWidth
		leftHeight += outer(l).Y + compareTooltipGap
	}
	headerHeight := outer(t.header).Y + compareTooltipGap
	colOuterWidth := outer(t.header).X
	size := geometry.Point{
		X: colOuterWidth + compareTooltipGap + outer(t.item).X,
		Y: max(leftHeight, headerHeight+outer(t.item).Y),
	}

	// centered above the mouse like a single tooltip, below it if there is no room, clamped to the screen
	const margin, mouseDist = 10, 50
	screen := u.GetScreenSize()
	x := mousePos.X - size.X/2
	y := mousePos.Y - size.Y - mouseDist
	if y < margin {
		y = mousePos.Y + mouseDist
	}
	x = max(min(x, screen.X-u.GetUIBarWidth()-20-size.X), margin)
	// ponytail: a group taller than the screen is cut off at the bottom, add scrolling if D1 tooltips ever get that long
	y = max(min(y, screen.Y-margin-size.Y), margin)

	pos := geometry.Point{X: x, Y: y}
	for _, l := range column {
		place(l, pos)
		pos.Y += outer(l).Y + compareTooltipGap
	}
	place(t.item, geometry.Point{X: x + colOuterWidth + compareTooltipGap, Y: y + headerHeight})
}

func (t *CompareTooltip) IsNull() bool { return false }

func (t *CompareTooltip) Draw() {
	t.header.Draw()
	for _, l := range t.left {
		l.Draw()
	}
	t.item.Draw()
}
