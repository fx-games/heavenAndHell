package gui

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image/color"
	"rpg/defs"
	"rpg/geometry"
	"rpg/util"
	"strconv"
)

type IconDataSource struct {
	IsHighlighted func(UIElement) bool
	GetElements   func() []UIElement
	OnSelection   func(UIElement)
	OnRightClick  func(UIElement)      // optional
	HotkeyHint    func(index int) rune // optional: a key drawn in the icon's top-left corner, 0 for none

	// Draggable icons are picked up on press (the UIController holds the drag) and a click acts on release.
	Draggable bool
	// OnDrop takes an icon dropped on slot to, dragged from fromIndex of the overlay from (maybe this one). Optional.
	OnDrop func(dragged UIElement, from *IconOverlay, fromIndex, to int)
	// OnDropOutside runs when an icon dragged from here is released anywhere but on this overlay. Optional.
	OnDropOutside func(fromIndex int)
}

type IconOverlay struct {
	infoLabel *BasicLabel

	getElements  func() []UIElement
	elements     []UIElement // what getElements returned at the last Refresh; nil entries are empty slots
	highlighted  []bool
	rows         int
	dimensions   geometry.Point
	margin       geometry.Point
	uiController *UIController
	topLeft      geometry.Point

	isHighlighted    func(UIElement) bool
	tileSizeInPixels geometry.Point
	bounds           geometry.Rect
	selectedIndex    int
	scale            float64

	showEmptySlots bool
	showShortCuts  bool
	showCounter    bool

	onSelection                 func(UIElement)
	onRightClick                func(UIElement)
	hotkeyHint                  func(index int) rune
	draggable                   bool
	onDrop                      func(dragged UIElement, from *IconOverlay, fromIndex, to int)
	onDropOutside               func(fromIndex int)
	isActive                    bool
	offsetFromBottomRightCorner geometry.Point
	tileSize                    geometry.Point
	onClose                     func()
	shouldClose                 bool
	canBeClosed                 bool
	infoLabelHandler            func(UIElement) string
	onCancel                    func()
}

func NewIconOverlay(uiController *UIController, offsetFromBottomRightCorner, tileSize geometry.Point, dataSource IconDataSource) *IconOverlay {
	i := &IconOverlay{
		uiController:                uiController,
		isHighlighted:               dataSource.IsHighlighted,
		getElements:                 dataSource.GetElements,
		onSelection:                 dataSource.OnSelection,
		onRightClick:                dataSource.OnRightClick,
		hotkeyHint:                  dataSource.HotkeyHint,
		draggable:                   dataSource.Draggable,
		onDrop:                      dataSource.OnDrop,
		onDropOutside:               dataSource.OnDropOutside,
		rows:                        4,
		selectedIndex:               -1,
		offsetFromBottomRightCorner: offsetFromBottomRightCorner,
		tileSize:                    tileSize,
	}
	i.infoLabel = NewCustomLabel(uiController, i.layoutLabel)
	i.Layout()
	i.Refresh()
	return i
}

// Refresh re-reads the icons from the data source; drawing and the mouse only ever look at what it read.
func (o *IconOverlay) Refresh() {
	o.elements = o.getElements()
	o.highlighted = make([]bool, len(o.elements))
	for i, e := range o.elements {
		o.highlighted[i] = e != nil && o.isHighlighted(e)
	}
}

// SetRows sets how many rows of eight icons the overlay has (4 by default).
func (o *IconOverlay) SetRows(rows int) {
	o.rows = rows
	o.Layout()
}

func (o *IconOverlay) SetInfoLabelHandler(handler func(UIElement) string) {
	o.infoLabelHandler = handler
}

func (o *IconOverlay) Layout() {
	scale := float64(4)
	tileSize := o.tileSize
	offsetFromBottomRightCorner := o.offsetFromBottomRightCorner
	screenSize := o.uiController.GetScreenSize()
	totalScale := scale

	tileSizeInPixels := geometry.Point{
		X: int(float64(tileSize.X) * totalScale),
		Y: int(float64(tileSize.Y) * totalScale),
	}
	dimensions := geometry.Point{X: 8, Y: o.rows}
	margin := geometry.Point{X: 1, Y: 1}

	width := dimensions.X*tileSizeInPixels.X + (dimensions.X-1)*margin.X
	height := dimensions.Y*tileSizeInPixels.Y + (dimensions.Y-1)*margin.Y
	// position contains the distance in x & axis from the bottom right corner of the screen
	bottomRight := geometry.Point{X: screenSize.X - offsetFromBottomRightCorner.X, Y: screenSize.Y - offsetFromBottomRightCorner.Y}
	topLeft := geometry.Point{X: bottomRight.X - width, Y: bottomRight.Y - height}

	o.offsetFromBottomRightCorner = offsetFromBottomRightCorner
	o.tileSize = tileSize

	o.topLeft = topLeft
	o.tileSizeInPixels = tileSizeInPixels
	o.dimensions = dimensions
	o.margin = margin
	o.bounds = geometry.NewRect(topLeft.X, topLeft.Y, topLeft.X+width, topLeft.Y+height)
	o.scale = scale

	o.layoutLabel(o.uiController, o.infoLabel)
}

func (o *IconOverlay) OnCommand(command CommandType) bool {
	if command == PlayerCommandCancel {
		if o.onCancel != nil {
			o.onCancel()
		}
		o.Close()
		return true
	}
	return false
}

func (o *IconOverlay) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
	if button == ebiten.MouseButtonRight && o.onRightClick != nil {
		point := geometry.Point{X: x, Y: y}
		if !o.bounds.Contains(point) {
			return false
		}
		if item, isValid := o.getSelectedItem(o.getItemIndexFromScreenPosition(point)); isValid {
			o.onRightClick(item)
			o.uiController.RefreshIconBars()
		}
		return true
	}
	if button == ebiten.MouseButtonRight {
		if o.onCancel != nil {
			o.onCancel()
		}
		if o.canBeClosed {
			o.Close()
			return true
		}
	}

	o.selectedIndex = -1
	point := geometry.Point{X: x, Y: y}
	if !o.bounds.Contains(point) {
		return false
	}
	o.selectedIndex = o.getItemIndexFromScreenPosition(point)
	if o.draggable {
		if item, isValid := o.getSelectedItem(o.selectedIndex); isValid {
			o.uiController.beginIconDrag(o, o.selectedIndex, item)
		}
		return true
	}
	o.onItemSelected(o.selectedIndex)
	return true
}

// onIconReleased ends a drag begun on this overlay's icon from: released on it, it acts; on another slot here or
// on another overlay, it is dropped there; anywhere else, it is dropped outside.
func (o *IconOverlay) onIconReleased(from int, dragged UIElement, target *IconOverlay, point geometry.Point) {
	switch {
	case target == o && o.getItemIndexFromScreenPosition(point) == from:
		o.onItemSelected(from)
	case target != nil && target.onDrop != nil:
		target.onDrop(dragged, o, from, target.getItemIndexFromScreenPosition(point))
	case target != o && o.onDropOutside != nil:
		o.onDropOutside(from)
	}
}

func (o *IconOverlay) OnMouseMoved(x int, y int) (bool, Tooltip) {
	o.selectedIndex = -1
	point := geometry.Point{X: x, Y: y}
	o.infoLabel.ClearText()
	if !o.bounds.Contains(point) {
		return false, NoTooltip{}
	}
	o.selectedIndex = o.getItemIndexFromScreenPosition(point)
	item, isValid := o.getSelectedItem(o.selectedIndex)
	if !isValid {
		return true, NoTooltip{}
	}
	infoLabelText := item.Name()

	if o.infoLabelHandler != nil {
		infoLabelText = o.infoLabelHandler(item)
	}

	o.infoLabel.SetText([]util.LabelText{
		{
			Text:      infoLabelText,
			TextColor: color.White,
		},
	})
	//o.layoutLabel(o.uiController, o.infoLabel)
	return true, NewElementTooltip(o.uiController, item, point)
}
func (o *IconOverlay) GetBounds() geometry.Rect {
	return o.bounds
}
func (o *IconOverlay) OnMouseWheel(x int, y int, dy float64) bool {
	return false
}

func (o *IconOverlay) ShouldClose() bool {
	return o.shouldClose
}

func (o *IconOverlay) Draw() {
	blackTransparent := color.RGBA{R: 0, G: 0, B: 0, A: 128}

	renderer := o.uiController.GetRenderer()
	items := o.elements
	for i := 0; i < o.dimensions.X*o.dimensions.Y; i++ {
		itemPosition := o.positionForItem(i)
		if i >= len(items) || items[i] == nil { // an empty slot
			if !o.showEmptySlots {
				continue
			}
			renderer.DrawDefaultBorder(itemPosition, o.tileSizeInPixels)
			//renderer.DrawOnScreenWithScale(itemPosition.X, itemPosition.Y, o.normalBackgroundIcon, o.scale)
			if i == o.selectedIndex {
				renderer.DrawOnScreenWithScale(itemPosition.X, itemPosition.Y, defs.IconTargeting, o.scale)
			}
			o.drawHotkeyHint(i, itemPosition)
			continue
		}
		item := items[i]
		gameItem := firstItemFromUIStack(item) // nil for a skill
		if o.highlighted[i] {
			renderer.DrawColoredBorder(itemPosition, o.tileSizeInPixels, defs.DarkGreen, defs.DarkYellow)
		} else {
			renderer.DrawDefaultBorder(itemPosition, o.tileSizeInPixels)
			//renderer.DrawOnScreenWithScale(itemPosition.X, itemPosition.Y, o.normalBackgroundIcon, o.scale)
		}
		if gameItem != nil && !gameItem.MeetsRequirements() {
			renderer.DrawColoredRect(itemPosition, o.tileSizeInPixels, color.RGBA{R: 75, G: 0, B: 0, A: 255})
		}
		renderer.DrawOnScreenWithScale(itemPosition.X, itemPosition.Y, item.InventoryIcon(), o.scale)
		if gameItem != nil {
			if percent, hasDurability := gameItem.DurabilityPercent(); hasDurability && percent < 0.5 {
				barHeight := 2
				barTopLeft := geometry.Point{X: itemPosition.X, Y: itemPosition.Y + o.tileSizeInPixels.Y - barHeight}
				barSize := geometry.Point{X: o.tileSizeInPixels.X, Y: barHeight}
				fillSize := geometry.Point{X: int(float64(o.tileSizeInPixels.X) * percent), Y: barHeight}
				renderer.DrawColoredRect(barTopLeft, barSize, color.RGBA{R: 255, G: 0, B: 0, A: 255}) // same colors as the player's health bar
				renderer.DrawColoredRect(barTopLeft, fillSize, color.RGBA{R: 0, G: 255, B: 0, A: 255})
			}
		}
		if i == o.selectedIndex {
			renderer.DrawOnScreenWithScale(itemPosition.X, itemPosition.Y, defs.IconTargeting, o.scale)
		}
		o.drawHotkeyHint(i, itemPosition)
		if o.showShortCuts {
			renderer.DrawCharOnScreen(itemPosition.X, itemPosition.Y, util.GetShortCutForIndex(i), 1, color.White)
		} else if o.showCounter {
			counter := item.Counter()
			if counter > 1 {
				counterString := strconv.Itoa(counter)
				_, cHeight := renderer.MeasureString(counterString)
				tileSize := o.tileSizeInPixels
				// NOTE: we have to specify the lower left corner where we want to start drawing.
				// in our case, that means we want to draw at the lower right corner of our tile
				// itemPosition is currently the top left corner

				drawPos := geometry.Point{
					X: itemPosition.X,
					Y: itemPosition.Y + tileSize.Y - 2,
				}
				borderTopLeft := geometry.Point{
					X: itemPosition.X,
					Y: itemPosition.Y + (tileSize.Y / 2),
				}
				borderWidth := tileSize.X //max(tileSize.X / 2, cWidth)
				borderHeight := max(tileSize.Y/2, int(cHeight))
				renderer.DrawColoredRect(borderTopLeft, geometry.Point{X: borderWidth, Y: borderHeight}, blackTransparent)
				renderer.DrawTTFOnScreen(float64(drawPos.X), float64(drawPos.Y), counterString, color.White)
			}
		}
	}
	o.infoLabel.Draw()
}

// drawHotkeyHint draws slot i's key in its top-left corner, empty slots included (the belt's keys are fixed).
func (o *IconOverlay) drawHotkeyHint(i int, itemPosition geometry.Point) {
	if o.hotkeyHint == nil || o.hotkeyHint(i) == 0 {
		return
	}
	renderer := o.uiController.GetRenderer()
	hint := string(o.hotkeyHint(i))
	const hintScale = 0.5
	w, h := renderer.MeasureString(hint)
	w, h = w*hintScale, h*hintScale
	renderer.DrawColoredRect(itemPosition, geometry.Point{X: int(w) + 2, Y: int(h) + 1}, color.RGBA{A: 128})
	renderer.DrawTTFOnScreenScaled(float64(itemPosition.X+1), float64(itemPosition.Y)+h, hint, color.White, hintScale)
}

func (o *IconOverlay) positionForItem(i int) geometry.Point {
	itemPosition := geometry.Point{
		X: o.topLeft.X + o.margin.X + (i%o.dimensions.X)*(o.tileSizeInPixels.X+o.margin.X),
		Y: o.topLeft.Y + o.margin.Y + (i/o.dimensions.X)*(o.tileSizeInPixels.Y+o.margin.Y),
	}
	return itemPosition
}

func (o *IconOverlay) getItemIndexFromScreenPosition(point geometry.Point) int {
	point.X -= o.topLeft.X
	point.Y -= o.topLeft.Y
	point.X /= o.tileSizeInPixels.X + o.margin.X
	point.Y /= o.tileSizeInPixels.Y + o.margin.Y
	return point.Y*o.dimensions.X + point.X
}

func (o *IconOverlay) onItemSelected(index int) {
	item, isValid := o.getSelectedItem(index)
	if !isValid {
		return
	}
	o.onSelection(item)
	o.uiController.RefreshIconBars()
}

func (o *IconOverlay) getSelectedItem(index int) (UIElement, bool) {
	if index >= len(o.elements) || index < 0 || o.elements[index] == nil {
		return nil, false
	}
	return o.elements[index], true
}

func (o *IconOverlay) IsActive() bool {
	return o.isActive
}

func (o *IconOverlay) Activate() {
	o.isActive = true
	o.showShortCuts = true
}

func (o *IconOverlay) Deactivate() {
	o.isActive = false
	o.showShortCuts = false
}

func (o *IconOverlay) HasItems() bool {
	return len(o.elements) > 0
}

func (o *IconOverlay) SetShowEmptySlots(value bool) {
	o.showEmptySlots = value
}

func (o *IconOverlay) SetShowCounter(value bool) {
	o.showCounter = value
}

func (o *IconOverlay) layoutLabel(controller *UIController, label *BasicLabel) {
	label.posInfo = SizeNeededForText(controller.GetRenderer().MeasureString, label.textLines, 0)
	label.topLeft = geometry.Point{
		X: o.topLeft.X, Y: o.topLeft.Y - label.posInfo.YOffsetForFirstLine - label.posInfo.SizeInPixels.Y + 4,
	}
	//        b.layoutFromTopLeftPos(topLeftPositionAsOffsetFromTopRight)
}

func (o *IconOverlay) GetOnSelectedHandler() func(uiItem UIElement) {
	return o.onSelection
}

func (o *IconOverlay) SetOnSelectedHandler(handler func(uiItem UIElement)) {
	o.onSelection = handler
}

func (o *IconOverlay) SetOnCloseHandler(onClose func()) {
	o.onClose = onClose
}

func (o *IconOverlay) SetCanBeClosed(canBeClosed bool) {
	o.canBeClosed = canBeClosed
}

func (o *IconOverlay) Close() {
	if o.shouldClose == true || !o.canBeClosed {
		return
	}
	o.shouldClose = true
	if o.onClose != nil {
		o.onClose()
	}
}

func (o *IconOverlay) SetOnCancelHandler(cancelHandler func()) {
	o.onCancel = cancelHandler
}
