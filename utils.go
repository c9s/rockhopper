package rockhopper

import (
	"regexp"
	"strings"
)

var (
	matchSQLComments       = regexp.MustCompile(`(?m)^--.*$[\r\n]*`)
	matchEmptyEOL          = regexp.MustCompile(`(?m)^$[\r\n]*`) // TODO: Duplicate
	matchNewLinesAndSpaces = regexp.MustCompile(`[ \r\n]+`)
)

// cleanSQL removes the SQL comments
func cleanSQL(s string) string {
	s = matchSQLComments.ReplaceAllString(s, "")
	return strings.TrimSpace(matchEmptyEOL.ReplaceAllString(s, ""))
}

func previewSQL(s string) string {
	s = matchNewLinesAndSpaces.ReplaceAllString(s, " ")

	width := 60
	if len(s) > width {
		idx := strings.LastIndex(s[:width], " ")
		if idx > 0 && idx > int(float64(width)*(2.0/3.0)) && idx < len(s)-3 {
			s = s[:idx] + "..."
			padSize := width - len(s)
			if padSize > 0 {
				s += strings.Repeat(" ", padSize)
			}
		
			return s
		}

		return s[:width]
	}

	return s + strings.Repeat(" ", width-len(s))
}
