package gui

import (
	"rpg/src/geometry"
	"testing"
)

// A layout taller than its space shows the rows that fit, follows the selection and never scrolls past either end,
// also after the content shrinks.
func TestRowScrolling(t *testing.T) {
	count := 20
	posInfo := PositionInfo{SizeInPixels: geometry.Point{Y: 20*10 + 19*5}, YOffsetForFirstLine: 10, LineDist: 5}
	rows := fitRows(&posInfo, count, 100)
	if rows != 7 || posInfo.SizeInPixels.Y != 7*15-5 {
		t.Fatalf("fit: %d rows, %d px", rows, posInfo.SizeInPixels.Y)
	}
	s := newRowScroller(func() int { return count }, &rows)
	if first, end := s.visible(count); first != 0 || end != 7 || s.canScrollUp() || !s.canScrollDown() {
		t.Fatalf("top: %d-%d", first, end)
	}
	s.MakeIndexVisible(19)
	if first, end := s.visible(count); first != 13 || end != 20 || s.canScrollDown() {
		t.Fatalf("bottom: %d-%d", first, end)
	}
	count = 5 // e.g. items sold: everything fits again
	s.keepOffsetInRange()
	if first, end := s.visible(count); first != 0 || end != 5 {
		t.Fatalf("after shrinking: %d-%d", first, end)
	}
	var zero ScrollingContent // widgets that never scroll
	if first, end := zero.visible(3); first != 0 || end != 3 {
		t.Fatalf("zero value: %d-%d", first, end)
	}
}
