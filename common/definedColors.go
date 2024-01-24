package common

import (
	"fmt"
	"image/color"
	"regexp"
	"rpg/util"
)

var PositiveHighlightColor = color.RGBA{R: 20, G: 255, B: 20, A: 255}
var NegativeHighlightColor = color.RGBA{R: 204, G: 034, A: 255}
var HeaderTextColor = color.RGBA{R: 212, G: 158, B: 67, A: 255}
var LightTextColor = color.RGBA{R: 224, G: 176, B: 33, A: 255}
var DarkTextColor = color.RGBA{R: 175, G: 131, B: 59, A: 255}
var TextBackgroundColor = color.RGBA{R: 0, G: 0, B: 0, A: 255}

var MagicHighlightColor = color.RGBA{R: 65, G: 105, B: 225, A: 255}
var RareHighlightColor = color.RGBA{R: 255, G: 255, A: 255}
var SetHighlightColor = color.RGBA{R: 0, G: 255, B: 0, A: 255}
var UniqueHighlightColor = color.RGBA{R: 165, G: 146, B: 99, A: 255}
var Red = color.RGBA{R: 255, G: 20, B: 20, A: 255}
var Green = color.RGBA{R: 20, G: 255, B: 20, A: 255}
var Blue = color.RGBA{R: 20, G: 20, B: 255, A: 255}
var White = color.RGBA{R: 255, G: 255, B: 255, A: 255}

var DarkGreen = color.RGBA{13 * 3, 30 * 3, 11 * 3, 255}

var Yellow = color.RGBA{R: 255, G: 255, B: 20, A: 255}

var DarkYellow = color.RGBA{R: 89, G: 83, B: 69, A: 255}

var PureYellow = color.RGBA{R: 255, G: 255, B: 0, A: 255}
var PureRed = color.RGBA{R: 255, G: 0, B: 0, A: 255}
var PureGreen = color.RGBA{R: 0, G: 255, B: 0, A: 255}
var PureBlue = color.RGBA{R: 0, G: 0, B: 255, A: 255}

func LabelTextFromString(text string) util.LabelText {
	return util.LabelText{Text: text, TextColor: DarkTextColor}
}
func LabelTextFromStringWithColor(text string, textColor color.Color) util.LabelText {
	return util.LabelText{Text: text, TextColor: textColor}
}

func ColorFromName(name string) color.Color {
	switch name {
	case "red":
		return Red
	case "blue":
		return Blue
	case "green":
		return Green
	case "yellow":
		return Yellow
	case "darkText":
		return DarkTextColor
	case "lightText":
		return LightTextColor
	default:
		return color.White
	}
}

func RemoveColorCodes(colorCodedText string) string {
	// color codes look like this [:red] [:blue] [:green] [:yellow] [:white] [:24,54,222]
	regexPattern := `\[:([\w,]+)\]`
	rp, _ := regexp.Compile(regexPattern)

	return rp.ReplaceAllString(colorCodedText, "")
}

func RGBAToColorCode(color color.RGBA) string {
	return fmt.Sprintf("[:%d,%d,%d]", color.R, color.G, color.B)
}
