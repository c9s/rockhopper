package rockhopper

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"
)

var goMigrationTemplate = template.Must(template.New("cmd.go-migration").Funcs(template.FuncMap{
	"quote": func(s string) string {
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + s + "\""
	},
}).Parse(`package {{.PackageName}}

import (
	"database/sql"
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(up{{.CamelName}}, down{{.CamelName}})
}

func up{{.CamelName}}(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is applied.
{{ range .Migration.UpStatements }}
	_, err = tx.ExecContext(ctx, {{ .SQL | quote }})
	if err != nil {
		return err
	}
{{ end }}
	return err
}

func down{{.CamelName}}(ctx context.Context, tx *sql.Tx) (err error) {
	// This code is executed when the migration is rolled back.
{{ range .Migration.DownStatements }}
	_, err = tx.ExecContext(ctx, {{ .SQL | quote }})
	if err != nil {
		return err
	}
{{ end }}
	return err
}
`))

type GoMigrationDumper struct {
	Dir         string
	PackageName string
}

func (d *GoMigrationDumper) Dump(m *Migration) error {
	packageName := d.PackageName
	if len(packageName) == 0 {
		packageName = filepath.Base(d.Dir)
	}

	buf := bytes.NewBuffer(nil)
	err := goMigrationTemplate.Execute(buf, struct {
		CamelName   string
		Migration   *Migration
		PackageName string
	}{
		CamelName:   strings.Title(toCamelCase(m.Name)),
		Migration:   m,
		PackageName: packageName,
	})
	if err != nil {
		return err
	}

	goFilename := filepath.Join(d.Dir, replaceExt(filepath.Base(m.Source), ".go"))
	return ioutil.WriteFile(goFilename, buf.Bytes(), 0666)
}
