package util

import (
	"image/color"
	"strings"
)

type MenuItem struct {
	MainText               string
	MainTextWithColorCodes []ColoredTextPart
	MainTextColor          color.Color

	UseColorCodes bool
	Action        func()
	CharIcon      int32
	TooltipText   []LabelText
	ActionLeft    func() string
	ActionRight   func() string
}

func (m MenuItem) String() string {
	return m.MainText
}

type LabelText struct {
	Text               string
	TextColor          color.Color
	UseColorCodes      bool
	TextWithColorCodes []ColoredTextPart
}

func (t LabelText) String() string {
	return t.Text
}

// PlainText is Text without [:color] markup.
func (t LabelText) PlainText() string {
	if !t.UseColorCodes || len(t.TextWithColorCodes) == 0 {
		return t.Text
	}
	var b strings.Builder
	for _, part := range t.TextWithColorCodes {
		b.WriteString(part.Text)
	}
	return b.String()
}

func (t LabelText) WithColor(textColor color.Color) LabelText {
	t.TextColor = textColor
	return t
}

func ToLabelText(buffer []string) []LabelText {
	var result []LabelText
	for _, line := range buffer {
		result = append(result, LabelTextFromString(line))
	}
	return result
}
func ToLabelTextWithColor(buffer []string, textColor color.Color) []LabelText {
	var result []LabelText
	for _, line := range buffer {
		result = append(result, LabelTextFromString(line).WithColor(textColor))
	}
	return result
}
func LabelTextFromString(text string) LabelText {
	return LabelText{Text: text, TextColor: color.White}
}
func LabelTextFromStringWithColor(text string, textColor color.Color) LabelText {
	return LabelText{Text: text, TextColor: textColor}
}
func IndexToXY(index int, width int) (int, int) {
	return index % width, index / width
}

func XYToIndex(x int, y int, width int) int {
	return y*width + x
}

// WrapLabelText greedily word-wraps plain lines wider than maxWidth pixels; color-coded lines pass through.
func WrapLabelText(lines []LabelText, maxWidth int, measureString func(line string) (float64, float64)) []LabelText {
	var result []LabelText
	for _, line := range lines {
		words := strings.Fields(line.Text)
		if w, _ := measureString(line.Text); line.UseColorCodes || len(words) == 0 || int(w) <= maxWidth {
			result = append(result, line)
			continue
		}
		current := words[0]
		for _, word := range words[1:] {
			if w, _ := measureString(current + " " + word); int(w) > maxWidth {
				result = append(result, LabelText{Text: current, TextColor: line.TextColor})
				current = word
			} else {
				current += " " + word
			}
		}
		result = append(result, LabelText{Text: current, TextColor: line.TextColor})
	}
	return result
}
