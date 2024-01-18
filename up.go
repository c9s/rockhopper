package rockhopper

import (
	"context"
	"fmt"
	"strings"

	"github.com/jedib0t/go-pretty/v6/text"
)

func UpBySteps(ctx context.Context, db *DB, m *Migration, steps int, callbacks ...func(m *Migration)) error {
	for ; steps > 0 && m != nil; m = m.Next {
		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}

		steps--
	}

	return nil
}

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
			"%2s %-12s %-6s >> %-28d (%d upgrade statements / %d downgrade statements)",
			char,
			strings.ToUpper(action),
			m.Package,
			m.Version,
			len(m.UpStatements), len(m.DownStatements),
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

func Up(ctx context.Context, db *DB, m *Migration, to int64, callbacks ...func(m *Migration)) error {
	for ; m != nil; m = m.Next {
		if to > 0 && m.Version > to {
			break
		}

		descMigration("upgrading", m)

		if err := m.Up(ctx, db); err != nil {
			return err
		}

		for _, cb := range callbacks {
			cb(m)
		}
	}

	return nil
}
