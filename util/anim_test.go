package util

import (
	"image/color"
	"testing"
)

func TestLerpColorDecreasingChannel(t *testing.T) {
	red := color.RGBA{R: 255, A: 255}
	blue := color.RGBA{B: 255, A: 255}
	for _, tc := range []struct {
		percent float64
		want    color.RGBA
	}{
		{0, red},
		{1, blue},
		{0.5, color.RGBA{R: 127, B: 127, A: 255}},
	} {
		if got := LerpColor(red, blue, tc.percent); got != color.Color(tc.want) {
			t.Errorf("percent %v: got %v want %v", tc.percent, got, tc.want)
		}
	}
}
