package ui

import (
    "github.com/hajimehoshi/ebiten/v2"
    "rpg/geometry"
    "strings"
)
type Keylistener interface {
    OnKeyPressed(key ebiten.Key, tick bool)
}
type IconList struct {
    uiController    *UIController
    isHighlighted   func(UIElement) bool
    getElements     func() []UIElement
    onSelection     func(rune)
    selectedIndex   int
    tileSize        geometry.Point
    needsScrolling  bool
    topLeft         geometry.Point
    borderPadding   int
    lineSpacing     float64
    posInfo         PositionInfo
    shouldClose     bool
    currentElements []UIElement
}

func (i *IconList) OnCommand(command CommandType) bool {
    if command == PlayerCommandCancel {
        i.Close()
    }
    return false
}
func (i *IconList) OnKeyPressed(key ebiten.Key, shiftPressed bool) {
    asString := key.String()
    if shiftPressed {
        asString = strings.ToUpper(asString)
    } else {
        asString = strings.ToLower(asString)
    }
    asRune := rune(asString[0])
    i.onSelection(asRune)
    i.Layout()
}
func (i *IconList) OnMouseClicked(button ebiten.MouseButton, x int, y int) bool {
    return false
}

func (i *IconList) OnMouseMoved(x int, y int) (bool, Tooltip) {
    return false, NoTooltip{}
}

func (i *IconList) OnMouseWheel(x int, y int, dy float64) bool {
    return false
}

func (i *IconList) Draw() {
    renderer := i.uiController.GetRenderer()

    renderer.DrawDefaultBorder(i.topLeft, i.posInfo.SizeInPixels)

    itemDrawX := i.topLeft.X + i.borderPadding
    itemDrawY := i.topLeft.Y + i.borderPadding + i.posInfo.YOffsetForFirstLine

    for eIndex, element := range i.currentElements {
        renderer.DrawTTFOnScreen(float64(itemDrawX), float64(itemDrawY), element.Name(), element.TintColor())
        itemDrawY += int(float64(i.posInfo.ItemSizes[eIndex].Y) + i.lineSpacing)
    }
}

func (i *IconList) ShouldClose() bool {
    return i.shouldClose
}

func (i *IconList) Layout() {
    i.updateElements()
    i.topLeft, i.posInfo = GetAutoFitRectWithExtraSpace(i.currentElements, i.uiController.GetScreenSize(), geometry.Point{X: i.borderPadding*2, Y: i.borderPadding*2}, i.uiController.GetRenderer().MeasureString)
}

func (i *IconList) updateElements() {
    i.currentElements = i.getElements()
    if len(i.currentElements) == 0 {
        i.Close()
    }
}

func (i *IconList) Close() {
    i.shouldClose = true
}

type ListDataSource struct {
    IsHighlighted func(UIElement) bool
    GetElements   func() []UIElement
    OnSelection   func(rune)
}


func NewIconList(uiController *UIController, tileSize geometry.Point, dataSource ListDataSource) *IconList {
    i := &IconList{
        uiController:                uiController,
        isHighlighted:               dataSource.IsHighlighted,
        getElements:                 dataSource.GetElements,
        onSelection:                 dataSource.OnSelection,
        selectedIndex:               -1,
        tileSize:                    tileSize,
        borderPadding:               5,
        lineSpacing:                 5.0,
    }

    //i.infoLabel = NewCustomLabel(uiController, i.layoutLabel)
    i.Layout()
    return i
}
