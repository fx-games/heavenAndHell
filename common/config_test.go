package common

import (
	"strings"
	"testing"
)

// TestMiniMapColorsRoundTrip guards the two things that can silently break:
// parsing the repeated MiniMapColor field, and writing it back on save.
func TestMiniMapColorsRoundTrip(t *testing.T) {
	colors := parseMiniMapColors("stairsDown=ffffff,stairsUp=#ffcc00, floor =4d3c66 ,bad=xyz,alsoBad")
	if len(colors) != 3 {
		t.Fatalf("want 3 parsed colors, got %d: %v", len(colors), colors)
	}
	if colors["stairsDown"] != [4]byte{255, 255, 255, 255} {
		t.Errorf("stairsDown = %v", colors["stairsDown"])
	}
	if colors["stairsUp"] != [4]byte{255, 204, 0, 255} {
		t.Errorf("stairsUp = %v", colors["stairsUp"])
	}
	if colors["floor"] != [4]byte{77, 60, 102, 255} {
		t.Errorf("floor = %v", colors["floor"])
	}

	var written []string
	for _, field := range (Configuration{MiniMapColors: colors}).ToRecord() {
		if field.Name == "MiniMapColor" {
			written = append(written, field.Value)
		}
	}
	if got := strings.Join(written, ","); got != "floor=4d3c66,stairsDown=ffffff,stairsUp=ffcc00" {
		t.Fatalf("ToRecord dropped or reordered colors: %q", got)
	}
	if again := parseMiniMapColors(strings.Join(written, ",")); len(again) != len(colors) {
		t.Fatalf("reparse lost colors: %v", again)
	}
}

// TestConfigRecParsesMiniMapColors catches a blank line or typo in the shipped config.rec.
func TestConfigRecParsesMiniMapColors(t *testing.T) {
	cfg := NewConfigurationFromRecFile("../config.rec")
	if len(cfg.MiniMapColors) == 0 {
		t.Fatal("config.rec MiniMapColor lines did not reach the configuration")
	}
	if cfg.MiniMapColors["stairsDown"] != [4]byte{255, 255, 255, 255} {
		t.Errorf("stairsDown = %v", cfg.MiniMapColors["stairsDown"])
	}
}
