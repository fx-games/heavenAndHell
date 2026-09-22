package app

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The F1 help is only useful while it stays complete, so this reads both sources: every key bound in
// gui/input.go must show up in the helpColumns table in gui/ui_controller.go. (A test inside package gui
// can't run: importing defs loads assets relative to the working directory.)
func TestHelpListsEveryBoundKey(t *testing.T) {
	input := readSource(t, "src/gui/input.go")
	helpTable := readSource(t, "src/gui/ui_controller.go")
	helpTable = helpTable[strings.Index(helpTable, "func helpColumns("):]
	helpTable = helpTable[:regexp.MustCompile(`(?m)^\}`).FindStringIndex(helpTable)[0]]

	listed := map[string]bool{}
	for _, entry := range regexp.MustCompile(`\{"([^"]+)", (?:"[^"]*"|\w+)\}`).FindAllStringSubmatch(helpTable, -1) {
		for _, token := range strings.FieldsFunc(entry[1], func(r rune) bool { return r == ' ' || r == '/' }) {
			listed[strings.ToLower(token)] = true
		}
	}
	if len(listed) < 20 {
		t.Fatalf("only found %d keys in helpColumns, the table moved or changed shape", len(listed))
	}

	modifiers := map[string]bool{"Shift": true, "Control": true, "Alt": true}
	alias := map[string]string{
		"Period": ".", "Escape": "esc",
		"Up": "arrows", "Down": "arrows", "Left": "arrows", "Right": "arrows",
		"ArrowUp": "arrows", "ArrowDown": "arrows", "ArrowLeft": "arrows", "ArrowRight": "arrows",
	}
	for _, match := range regexp.MustCompile(`ebiten\.Key(\w+)`).FindAllStringSubmatch(input, -1) {
		key := match[1]
		if modifiers[key] {
			continue
		}
		want, ok := alias[key]
		if !ok {
			want = strings.ToLower(key)
		}
		if !listed[want] {
			t.Errorf("gui/input.go binds ebiten.Key%s, but the F1 help never mentions %q", key, want)
		}
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	source, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(source)
}
