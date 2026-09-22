package textui

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/fxtools"
	"github.com/memmaker/go/textiles"
)

func RightPadColored(s string, pLen int) string {
	width := cview.TaggedStringWidth(s)
	return s + strings.Repeat(" ", max(0, pLen-width))
}

func expandToWidth(statusStr string, width int) string {
	statusWidth := cview.TaggedStringWidth(statusStr)
	if statusWidth < width {
		statusStr = fxtools.RightPadCount(statusStr, width-statusWidth)
	}
	return statusStr
}

func joinLines(lines []string) string { return strings.Join(lines, "\n") }

func applyGamma(colorChannel uint8, gamma float64) uint8 {
	colorAsFloat := float64(colorChannel) / 255.0
	gammaCorrected := fxtools.Clamp(0, 1, math.Pow(colorAsFloat, gamma))
	return uint8(gammaCorrected * 255.0)
}

func tcellColorToRGBA(tColor tcell.Color) color.RGBA {
	rF, gF, bF := tColor.RGB()
	return color.RGBA{R: uint8(rF), G: uint8(gF), B: uint8(bF), A: 255}
}

func IconAsString(icon textiles.TextIcon) string {
	code := textiles.RGBAToColorCodes(icon.Fg, icon.Bg)
	return fmt.Sprintf("%s%s[-:-]", code, string(icon.Char))
}

// RuneBarFromPercent is a bar of icons, colored by the first stage color above 75%, the second above 50%, the third below.
func RuneBarFromPercent(icon rune, percent float64, width int, stageColors [3]color.RGBA) string {
	repeats := min(width, max(0, int(math.Round(percent*float64(width)))))
	colorCode := textiles.RGBAToFgColorCode(stageColors[0])
	if percent < 0.50 {
		colorCode = textiles.RGBAToFgColorCode(stageColors[2])
	} else if percent < 0.75 {
		colorCode = textiles.RGBAToFgColorCode(stageColors[1])
	}
	return colorCode + strings.Repeat(string(icon), repeats) + "[-]" + strings.Repeat(" ", width-repeats)
}

func desaturate(fg color.RGBA) color.RGBA {
	gray := uint8((uint16(fg.R) + uint16(fg.G) + uint16(fg.B)) / 3)
	return color.RGBA{R: gray, G: gray, B: gray, A: fg.A}
}

func darken(fg color.RGBA) color.RGBA {
	return color.RGBA{R: fg.R / 2, G: fg.G / 2, B: fg.B / 2, A: fg.A}
}

func toTcellColor(rgba color.RGBA) tcell.Color {
	return tcell.NewRGBColor(int32(rgba.R), int32(rgba.G), int32(rgba.B))
}

func widthAndHeightFromString(description string) (int, int) {
	longest := 0
	height := 0
	for _, line := range strings.Split(description, "\n") {
		longest = max(longest, cview.TaggedStringWidth(line))
		height++
	}
	return longest, height
}

// drawBackgroundAndBorderWithTitle fills a box, draws its border and centers a title on the top edge.
func drawBackgroundAndBorderWithTitle(screen tcell.Screen, x int, y int, width int, height int, title string, style tcell.Style, runes []rune) {
	horizontal, vertical := runes[0], runes[1]
	topLeft, topRight, bottomRight, bottomLeft := runes[2], runes[3], runes[4], runes[5]
	put := func(r rune, atX, atY int) { screen.SetContent(atX, atY, r, nil, style) }

	for i := x; i < x+width; i++ {
		for j := y; j < y+height; j++ {
			put(' ', i, j)
		}
	}

	put(topLeft, x, y)
	put(topRight, x+width-1, y)
	put(bottomRight, x+width-1, y+height-1)
	put(bottomLeft, x, y+height-1)

	titleRunes := []rune(title)
	startTitleX := x + 1 + (width-2-len(titleRunes))/2
	for i := x + 1; i < x+width-1; i++ {
		if len(titleRunes) > 0 && i >= startTitleX && i < startTitleX+len(titleRunes) {
			put(titleRunes[i-startTitleX], i, y)
		} else {
			put(horizontal, i, y)
		}
		put(horizontal, i, y+height-1)
	}

	for i := y + 1; i < y+height-1; i++ {
		put(vertical, x, i)
		put(vertical, x+width-1, i)
	}
}
