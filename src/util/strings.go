package util

import (
	"fmt"
	"strings"
)

func RightPad(s string, pLen int) string {
	return s + strings.Repeat(" ", pLen-len(s))
}

func rightAlignColumns(cols []string, width []int) string {
	for i, col := range cols {
		if len(col) < width[i] {
			cols[i] = fmt.Sprintf("%s%s", strings.Repeat(" ", width[i]-len(col)), col)
		}
	}
	return strings.Join(cols, "")
}

type TableRow struct {
	Label   string
	Columns []string
}

func TableLayout(tableData []TableRow) []string {
	var maxLabelLen int
	colWidths := make([]int, len(tableData[0].Columns))

	for _, row := range tableData {
		if len(row.Label) > maxLabelLen {
			maxLabelLen = len(row.Label)
		}
		for i, col := range row.Columns {
			if len(col)+1 > colWidths[i] {
				colWidths[i] = len(col) + 1
			}
		}
	}

	var result []string
	for _, row := range tableData {
		result = append(result, fmt.Sprintf("%s%s", RightPad(row.Label, maxLabelLen+1), rightAlignColumns(row.Columns, colWidths)))
	}
	return result
}

func GetShortCutForIndex(i int) rune {
	if i < 9 {
		return rune('1' + i)
	}
	// else start with a
	return rune('A' + i - 9)
}
