package rockhopper

import (
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/text"
)

// descMigration prints out the migration info in a fancy format
func descMigration(action string, m *Migration) {
	char := "\u21E1"
	colors := text.Colors{text.FgBlack, text.BgHiGreen}
	switch action {
	case "downgrading":
		colors = text.Colors{text.FgBlack, text.BgHiCyan}
		char = "\u21E3"
	case "upgrading":
		char = "\u21E1"
		colors = text.Colors{text.FgBlack, text.BgHiGreen}
	}

	fmt.Printf(
		colors.Sprintf(
			"%2s %-12s %-6s >> %-28d (%d upgrade statements / %d downgrade statements) %2s",
			strings.Repeat(char, 2),
			strings.ToUpper(action),
			m.Package,
			m.Version,
			len(m.UpStatements), len(m.DownStatements),
			strings.Repeat(char, 2),
		))
	fmt.Print("\n")

	/*
		fmt.Printf("%s %s > %s  (%d upgrade statements / %d downgrade statements)\n",
			text.Colors{text.FgHiWhite, text.BgGreen}.Sprint("MIGRATION"),
			text.Colors{text.FgGreen}.Sprint(m.Package),
			text.Colors{text.FgGreen}.Sprint(m.Version),
			len(m.UpStatements), len(m.DownStatements),
		)
	*/
}
