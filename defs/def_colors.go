package defs

import (
	"fmt"
	"image/color"
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
var InvColorUnique = color.RGBA{R: 45, G: 255, B: 254, A: 255}
var InvColorLightSource = color.RGBA{R: 255, G: 253, B: 56, A: 255}
var InvColorArmor = color.RGBA{R: 191, G: 128, B: 70, A: 255}
var InvColorMisc = color.RGBA{R: 191, G: 128, B: 70, A: 255}
var InvColorMeleeWeapon = color.RGBA{R: 230, G: 230, B: 230, A: 255}
var InvColorPotion = color.RGBA{R: 45, G: 255, B: 254, A: 255}
var InvColorScroll = color.RGBA{R: 240, G: 240, B: 240, A: 255}
var InvColorWand = color.RGBA{R: 16, G: 127, B: 66, A: 255}
var InvColorRing = color.RGBA{R: 190, G: 7, B: 18, A: 255}
var InvColorAmulet = color.RGBA{R: 249, G: 129, B: 40, A: 255}
var InvColorRangedWeapon = color.RGBA{R: 127, G: 64, B: 12, A: 255}
var Red = color.RGBA{R: 255, G: 20, B: 20, A: 255}
var Green = color.RGBA{R: 20, G: 255, B: 20, A: 255}
var Blue = color.RGBA{R: 20, G: 20, B: 255, A: 255}

var Brown = color.RGBA{R: 209, G: 145, B: 33, A: 255}
var LightGray = color.RGBA{R: 190, G: 190, B: 190, A: 255}
var DarkGray = color.RGBA{R: 111, G: 111, B: 111, A: 255}
var Black = color.RGBA{R: 0, G: 0, B: 0, A: 255}

var Cyan = color.RGBA{R: 10, G: 242, B: 231, A: 255}
var White = color.RGBA{R: 255, G: 255, B: 255, A: 255}

var OffWhite = color.RGBA{R: 200, G: 200, B: 200, A: 255}

var DarkGreen = color.RGBA{13 * 3, 30 * 3, 11 * 3, 255}

var Yellow = color.RGBA{R: 255, G: 255, B: 20, A: 255}

var DarkYellow = color.RGBA{R: 89, G: 83, B: 69, A: 255}

var PureYellow = color.RGBA{R: 255, G: 255, B: 0, A: 255}
var PureRed = color.RGBA{R: 255, G: 0, B: 0, A: 255}
var PureGreen = color.RGBA{R: 0, G: 255, B: 0, A: 255}
var PureBlue = color.RGBA{R: 0, G: 0, B: 255, A: 255}

func ColorFromName(name string) color.RGBA {
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
	case "magic":
		return MagicHighlightColor
	case "rare":
		return RareHighlightColor
	case "set":
		return SetHighlightColor
	case "unique":
		return InvColorUnique
	case "positive":
		return PositiveHighlightColor
	case "negative":
		return NegativeHighlightColor
	default:
		return White
	}
}
func ColorCodeFromColor(color color.Color) string {
	switch color {
	case Red:
		return "[:red]"
	case Blue:
		return "[:blue]"
	case Green:
		return "[:green]"
	case Yellow:
		return "[:yellow]"
	case DarkTextColor:
		return "[:darkText]"
	case LightTextColor:
		return "[:lightText]"
	case MagicHighlightColor:
		return "[:magic]"
	case RareHighlightColor:
		return "[:rare]"
	case SetHighlightColor:
		return "[:set]"
	case InvColorUnique:
		return "[:unique]"
	default:
		return "[:white]"
	}
}

func RGBAToColorCode(color color.RGBA) string {
	return fmt.Sprintf("[:%d,%d,%d]", color.R, color.G, color.B)
}
