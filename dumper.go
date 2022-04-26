package rockhopper

import (
	"bytes"
	"go/format"
	"io/ioutil"
	"path/filepath"
	"strings"
	"text/template"
)

var templateFuncs = template.FuncMap{
	"quote": func(s string) string {
		s = strings.ReplaceAll(s, "\n", "\\n")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		return "\"" + s + "\""
	},
}

var apiTemplate = template.Must(
	template.New("cmd.go-migration-api").
		Funcs(templateFuncs).
		Parse(`package {{.PackageName}}

import (
	"runtime"
	"strings"
	"log"
	"fmt"

	"github.com/c9s/rockhopper"
)

var registeredGoMigrations map[int64]*rockhopper.Migration

func MergeMigrationsMap(ms map[int64]*rockhopper.Migration) {
	for k, m := range ms {
		if _, ok := registeredGoMigrations[k] ; !ok {
			registeredGoMigrations[k] = m
		} else {
			log.Printf("the migration key %d is duplicated: %+v", k, m)
		}
	}
}

func GetMigrationsMap() map[int64]*rockhopper.Migration {
	return registeredGoMigrations
}

// SortedMigrations builds up the migration objects, sort them by timestamp and return as a slice
func SortedMigrations() rockhopper.MigrationSlice {
	return Migrations()
}

// Migrations builds up the migration objects, sort them by timestamp and return as a slice
func Migrations() rockhopper.MigrationSlice {
	var migrations = rockhopper.MigrationSlice{}
	for _, migration := range registeredGoMigrations {
		migrations = append(migrations, migration)
	}

	return migrations.SortAndConnect()
}

// AddMigration adds a migration.
func AddMigration(up, down rockhopper.TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	funcName := runtime.FuncForPC(pc).Name()
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}
	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	AddNamedMigration(packageName, filename, up, down)
}

// AddNamedMigration : Add a named migration.
func AddNamedMigration(packageName, filename string, up, down rockhopper.TransactionHandler) {
	if registeredGoMigrations == nil {
		registeredGoMigrations = make(map[int64]*rockhopper.Migration)
	}

	v, _ := rockhopper.FileNumericComponent(filename)

	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	if existing, ok := registeredGoMigrations[v]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with %q", filename, existing.Source))
	}
	registeredGoMigrations[v] = migration
}

`))

var migrationTemplate = template.Must(template.New("cmd.go-migration").Funcs(templateFuncs).Parse(`package {{.PackageName}}

import (
	"context"

	"github.com/c9s/rockhopper"
)

func init() {
	AddMigration(up{{.CamelName}}, down{{.CamelName}})

{{ if .Global }}
	rockhopper.AddMigration(up{{.CamelName}}, down{{.CamelName}})
{{ end }}
}

func up{{.CamelName}}(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
{{ range .Migration.UpStatements }}
	_, err = tx.ExecContext(ctx, {{ .SQL | quote }})
	if err != nil {
		return err
	}
{{ end }}
	return err
}

func down{{.CamelName}}(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
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

type apiTemplateArgs struct {
	PackageName string
}

func renderMigrationApi(packageName string) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := apiTemplate.Execute(buf, apiTemplateArgs{
		PackageName: packageName,
	})
	if err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

type migrationTemplateArgs struct {
	CamelName   string

	// BaseName is the file basename of the migration script.
	BaseName    string

	// Migration is the Migration metadata object
	Migration   *Migration

	// PackageName is the package name that will be used to render the go file.
	PackageName string

	// Global renders the migration template with the global migration registration calls.
	// This parameter avoids migration version conflict for different sql dialect (or driver)
	Global      bool
}

func renderMigration(packageName string, m *Migration) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := migrationTemplate.Execute(buf, migrationTemplateArgs{
		CamelName:   strings.Title(toCamelCase(m.Name)),
		BaseName:    filepath.Base(m.Source),
		Migration:   m,
		PackageName: packageName,
	})
	if err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

type GoMigrationDumper struct {
	Dir         string
	PackageName string
}

func (d *GoMigrationDumper) DumpApi() error {
	packageName := d.PackageName
	if len(packageName) == 0 {
		packageName = filepath.Base(d.Dir)
	}

	out, err := renderMigrationApi(packageName)
	if err != nil {
		return err
	}

	goFilename := filepath.Join(d.Dir, "migration_api.go")
	return ioutil.WriteFile(goFilename, out, 0666)
}

func (d *GoMigrationDumper) Dump(migrations MigrationSlice) error {
	if err := d.DumpApi() ; err != nil {
		return err
	}

	for _, migration := range migrations {
		err := d.DumpMigration(migration)
		if err != nil {
			return err
		}
	}

	return nil
}


func (d *GoMigrationDumper) DumpMigration(m *Migration) error {
	packageName := d.PackageName
	if len(packageName) == 0 {
		packageName = filepath.Base(d.Dir)
	}

	out, err := renderMigration(packageName, m)
	if err != nil {
		return err
	}

	goFilename := filepath.Join(d.Dir, replaceExt(filepath.Base(m.Source), ".go"))
	return ioutil.WriteFile(goFilename, out, 0666)
}
