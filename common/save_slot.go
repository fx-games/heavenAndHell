package common

import (
	"fmt"
	"time"
)

// SaveSlot is one save slot as the load/save screen shows it: enough to tell two heroes apart without loading
// either. An unused slot carries nothing but its number.
type SaveSlot struct {
	Slot       int
	Used       bool
	Name       string
	Class      string
	Level      int
	Deepest    int // the deepest dungeon level reached
	Permadeath bool
	SavedAt    time.Time
	ThumbPath  string // the screenshot saved beside the game, empty when there is none
}

// Where names the deepest place the hero got to, the way the game talks about it.
func (s SaveSlot) Where() string {
	switch {
	case s.Deepest <= 1:
		return "Tristram"
	case s.Deepest <= 4:
		return fmt.Sprintf("Cathedral %d", s.Deepest)
	case s.Deepest <= 8:
		return fmt.Sprintf("Catacombs %d", s.Deepest)
	case s.Deepest <= 12:
		return fmt.Sprintf("Caves %d", s.Deepest)
	}
	return fmt.Sprintf("Hell %d", s.Deepest)
}

// Rows are the slot's metadata as label/value pairs, so both UIs list the same facts in the same order.
func (s SaveSlot) Rows() [][2]string {
	if !s.Used {
		return nil
	}
	rows := [][2]string{
		{"Hero", fmt.Sprintf("%s the %s", s.Name, s.Class)},
		{"Level", fmt.Sprintf("%d", s.Level)},
		{"Deepest", s.Where()},
		{"Played", s.SavedAt.Local().Format("2006-01-02 15:04")},
	}
	if s.Permadeath {
		rows = append(rows, [2]string{"Mode", "Permadeath"})
	}
	return rows
}

// Summary is the slot on one line, for menus and confirmation prompts.
func (s SaveSlot) Summary() string {
	if !s.Used {
		return fmt.Sprintf("%d. empty", s.Slot)
	}
	return fmt.Sprintf("%d. %s the %s, level %d, %s, %s", s.Slot, s.Name, s.Class, s.Level, s.Where(),
		s.SavedAt.Local().Format("2006-01-02 15:04"))
}
