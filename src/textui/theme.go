package textui

import (
	"image/color"
	"io"
	"log/slog"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/memmaker/go/cview"
	"github.com/memmaker/go/recfile"
	"github.com/memmaker/go/textiles"
)

type Theme struct {
	uiColors map[UIColor]color.RGBA

	uiBorder     map[BorderCases]rune
	defaultStyle tcell.Style

	palette textiles.ColorPalette
}

// NewUIThemeFromRecords reads a ui.rec with a %rec: ui record of palette color names and a %rec: borders record.
func NewUIThemeFromRecords(uiRec io.Reader, palette textiles.ColorPalette) Theme {
	records, _ := recfile.ReadMulti(uiRec)

	uiBorders := loadBorders(records["borders"][0])
	uiColors := loadUIColors(records["ui"][0], palette)

	var defaultStyle tcell.Style
	defaultStyle = defaultStyle.Foreground(toTcellColor(uiColors[UIColorUIForeground])).Background(toTcellColor(uiColors[UIColorUIBackground]))

	return Theme{
		palette:      palette,
		uiColors:     uiColors,
		uiBorder:     uiBorders,
		defaultStyle: defaultStyle,
	}
}

func (t Theme) GetUIColor(foreground UIColor) color.RGBA {
	return t.uiColors[foreground]
}

func (t Theme) GetUIColorForTcell(foreground UIColor) tcell.Color {
	return toTcellColor(t.uiColors[foreground])
}

func loadUIColors(record recfile.Record, palette textiles.ColorPalette) map[UIColor]color.RGBA {
	uiColors := make(map[UIColor]color.RGBA)
	for _, field := range record {
		uiColor := palette.Get(field.Value)
		if uiColor.A == 0 { // not in the palette: better a visible default than black on black
			slog.Warn("no palette color", "name", field.Value)
			uiColor = color.RGBA{R: 200, G: 200, B: 200, A: 255}
		}
		uiColors[UIColorFromString(field.Name)] = uiColor
	}
	return uiColors
}

func loadBorders(record recfile.Record) map[BorderCases]rune {
	borders := make(map[BorderCases]rune)
	for _, field := range record {
		borders[BorderCaseFromString(strings.TrimPrefix(field.Name, "UIBorder_"))] = field.AsRune()
	}
	return borders
}

type UIColor int

const (
	UIColorUIBackground UIColor = iota
	UIColorUIForeground
	UIColorBorderBackground
	UIColorBorderForeground
	UIColorBorderForegroundFocus
	UIColorTextForegroundHighlighted
	UIColorStatusBarBackground
	UIColorStatusBarForeground
)

func UIColorFromString(s string) UIColor {
	switch strings.ToLower(s) {
	case "uiforeground":
		return UIColorUIForeground
	case "uiforegroundhighlighted":
		return UIColorTextForegroundHighlighted
	case "uibackground":
		return UIColorUIBackground
	case "borderbackground":
		return UIColorBorderBackground
	case "borderforeground":
		return UIColorBorderForeground
	case "borderforegroundfocus":
		return UIColorBorderForegroundFocus
	case "statusbarbackground":
		return UIColorStatusBarBackground
	case "statusbarforeground":
		return UIColorStatusBarForeground
	}
	slog.Warn("unknown color", "name", s)
	return UIColorUIForeground
}

type BorderCases int

const (
	BorderHorizontal BorderCases = iota
	BorderVertical
	BorderTopLeft
	BorderTopRight
	BorderBottomLeft
	BorderBottomRight
	BorderLeftT
	BorderRightT
	BorderTopT
	BorderBottomT
	BorderCross
	BorderHorizontalFocus
	BorderVerticalFocus
	BorderTopLeftFocus
	BorderTopRightFocus
	BorderBottomLeftFocus
	BorderBottomRightFocus
)

var borderNames = map[string]BorderCases{
	"horizontal": BorderHorizontal, "vertical": BorderVertical, "topleft": BorderTopLeft, "topright": BorderTopRight,
	"bottomleft": BorderBottomLeft, "bottomright": BorderBottomRight, "leftt": BorderLeftT, "rightt": BorderRightT,
	"topt": BorderTopT, "bottomt": BorderBottomT, "cross": BorderCross, "horizontalfocus": BorderHorizontalFocus,
	"verticalfocus": BorderVerticalFocus, "topleftfocus": BorderTopLeftFocus, "toprightfocus": BorderTopRightFocus,
	"bottomleftfocus": BorderBottomLeftFocus, "bottomrightfocus": BorderBottomRightFocus,
}

func BorderCaseFromString(s string) BorderCases {
	if border, ok := borderNames[strings.ToLower(s)]; ok {
		return border
	}
	slog.Warn("unknown border case", "name", s)
	return BorderHorizontal
}

func (t Theme) SetBorders(s *cview.BorderDef) {
	s.Horizontal = t.uiBorder[BorderHorizontal]
	s.Vertical = t.uiBorder[BorderVertical]
	s.TopLeft = t.uiBorder[BorderTopLeft]
	s.TopRight = t.uiBorder[BorderTopRight]
	s.BottomLeft = t.uiBorder[BorderBottomLeft]
	s.BottomRight = t.uiBorder[BorderBottomRight]
	s.LeftT = t.uiBorder[BorderLeftT]
	s.RightT = t.uiBorder[BorderRightT]
	s.TopT = t.uiBorder[BorderTopT]
	s.BottomT = t.uiBorder[BorderBottomT]
	s.Cross = t.uiBorder[BorderCross]
	s.HorizontalFocus = t.uiBorder[BorderHorizontalFocus]
	s.VerticalFocus = t.uiBorder[BorderVerticalFocus]
	s.TopLeftFocus = t.uiBorder[BorderTopLeftFocus]
	s.TopRightFocus = t.uiBorder[BorderTopRightFocus]
	s.BottomLeftFocus = t.uiBorder[BorderBottomLeftFocus]
	s.BottomRightFocus = t.uiBorder[BorderBottomRightFocus]
}

func (t Theme) GetMapDefaultStyle() tcell.Style {
	return t.defaultStyle
}

func (t Theme) GetColorByName(name string) color.RGBA {
	return t.palette.Get(name)
}

// SetStyles points cview's own defaults at the theme, so that widgets we don't style by hand - modals, forms,
// buttons, input fields - are readable too instead of using cview's white on black and green.
func (t Theme) SetStyles(s *cview.Theme) {
	fg := toTcellColor(t.uiColors[UIColorUIForeground])
	highlighted := toTcellColor(t.uiColors[UIColorTextForegroundHighlighted])
	background := toTcellColor(t.uiColors[UIColorUIBackground])
	border := toTcellColor(t.uiColors[UIColorBorderForeground])

	s.TitleColor = highlighted
	s.BorderColor = border
	s.GraphicsColor = border

	s.PrimaryTextColor = fg
	s.SecondaryTextColor = highlighted
	s.TertiaryTextColor = fg
	s.InverseTextColor = background
	s.ContrastPrimaryTextColor = background
	s.ContrastSecondaryTextColor = background

	s.PrimitiveBackgroundColor = background
	s.ContrastBackgroundColor = highlighted // e.g. the focused button
	s.MoreContrastBackgroundColor = fg
}
