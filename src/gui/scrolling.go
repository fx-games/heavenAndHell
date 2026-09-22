package gui

import (
	"rpg/src/defs"
	"rpg/src/geometry"
	"rpg/src/renderer"
)

// ScrollingContent scrolls content taller than its space, in whole lines (list rows, text lines).
type ScrollingContent struct {
	scrollOffset           float64
	neededHeightForContent func() int
	availableSpace         func() geometry.Rect
}

func NewScrollingContentWithFunctions(neededHeight func() int, availableSpace func() geometry.Rect) ScrollingContent {
	return ScrollingContent{
		neededHeightForContent: neededHeight,
		availableSpace:         availableSpace,
	}
}
func (s *ScrollingContent) ScrollDown(amount int) {
	if !s.canScrollDown() {
		return
	}
	s.scrollOffset += float64(amount)
	s.keepOffsetInRange()
}

func (s *ScrollingContent) ScrollUp(amount int) {
	if !s.canScrollUp() {
		return
	}
	s.scrollOffset -= float64(amount)
	s.keepOffsetInRange()
}

func (s *ScrollingContent) availableHeightForContent() int {
	size := s.availableSpace().Size()
	return size.Y
}

func (s *ScrollingContent) gridScrollOffset() int {
	return int(s.scrollOffset)
}
func (s *ScrollingContent) maxScrollOffset() float64 {
	return float64(s.neededHeightForContent() - s.availableHeightForContent())
}
func (s *ScrollingContent) OnMouseWheel(x int, y int, dy float64) bool {
	if !s.needsScroll() {
		return false
	}
	sensitity := 0.2
	s.scrollOffset += -dy * sensitity
	s.keepOffsetInRange()
	return true
}

func (s *ScrollingContent) keepOffsetInRange() {
	s.scrollOffset = max(0, min(s.scrollOffset, s.maxScrollOffset()))
}

func (s *ScrollingContent) canScrollUp() bool {
	if !s.needsScroll() {
		return false
	}
	return s.scrollOffset > 0
}
func (s *ScrollingContent) canScrollDown() bool {
	if !s.needsScroll() {
		return false
	}
	return s.gridScrollOffset() < s.neededHeightForContent()-s.availableHeightForContent()
}
func (s *ScrollingContent) needsScroll() bool {
	if s.neededHeightForContent == nil { // zero value: nothing to scroll
		return false
	}
	neededHeightForContent := s.neededHeightForContent()
	availableHeightForContent := s.availableHeightForContent()
	return neededHeightForContent > availableHeightForContent
}

func (s *ScrollingContent) ScrollToTop() {
	s.scrollOffset = 0
}

func (s *ScrollingContent) ScrollToBottom() {
	if !s.needsScroll() {
		return
	}
	s.scrollOffset = s.maxScrollOffset()
}

func (s *ScrollingContent) MakeIndexVisible(selectionIndex int) {
	if !s.needsScroll() {
		return
	}
	if selectionIndex < s.gridScrollOffset() {
		s.scrollOffset = float64(selectionIndex)
	} else if selectionIndex >= s.gridScrollOffset()+s.availableHeightForContent() {
		s.scrollOffset = float64(selectionIndex - s.availableHeightForContent() + 1)
	}
}

// visible is the half-open range of rows on screen out of count.
func (s *ScrollingContent) visible(count int) (first, end int) {
	if !s.needsScroll() {
		return 0, count
	}
	first = min(s.gridScrollOffset(), count)
	return first, min(first+s.availableHeightForContent(), count)
}

// newRowScroller scrolls count() rows, *rows of which fit (fitRows sets it).
func newRowScroller(count func() int, rows *int) ScrollingContent {
	return NewScrollingContentWithFunctions(count, func() geometry.Rect { return geometry.NewRect(0, 0, 0, *rows) })
}

// fitRows caps a line layout at maxHeight pixels: it returns how many of the count rows show, and shrinks
// posInfo's height to them. Rows are taken as one line high, like SizeNeededForText lays out blank lines.
func fitRows(posInfo *PositionInfo, count, maxHeight int) int {
	rowHeight := posInfo.YOffsetForFirstLine + posInfo.LineDist
	if posInfo.SizeInPixels.Y <= maxHeight || rowHeight <= 0 {
		return count
	}
	rows := max(1, (maxHeight+posInfo.LineDist)/rowHeight)
	posInfo.SizeInPixels.Y = rows*rowHeight - posInfo.LineDist
	return rows
}

// drawArrows marks that there is more above (at topY) or below (at bottomY), as text baselines at x.
// ASCII, not ▲▼: several of the mods' fonts have no arrows.
func (s *ScrollingContent) drawArrows(r *renderer.TileRenderer, x, topY, bottomY int) {
	if s.canScrollUp() {
		r.DrawTTFOnScreen(float64(x), float64(topY), "^", defs.LightTextColor)
	}
	if s.canScrollDown() {
		r.DrawTTFOnScreen(float64(x), float64(bottomY), "v", defs.LightTextColor)
	}
}
