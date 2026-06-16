package rockhopper

import (
	"bytes"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
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

var testTemplate = template.Must(
	template.New("cmd.go-migration-api").
		Funcs(templateFuncs).
		Parse(`package {{.PackageName}}

import (
	"testing"

	"github.com/c9s/rockhopper/v2"

	"github.com/stretchr/testify/assert"
)

func TestGetMigrationsMap(t *testing.T) {
	mm := GetMigrationsMap()
	assert.NotEmpty(t, mm)
}

func TestMergeMigrationsMap(t *testing.T) {
	MergeMigrationsMap(map[rockhopper.RegistryKey]*rockhopper.Migration{
		{Version: 2}: {},
		{Version: 3}: {},
	})
}

`))

var apiTemplate = template.Must(
	template.New("cmd.go-migration-api").
		Funcs(templateFuncs).
		Parse(`package {{.PackageName}}

import (
	"runtime"
	"strings"
	"log"
	"fmt"

	"github.com/c9s/rockhopper/v2"
)

var registeredGoMigrations = map[rockhopper.RegistryKey]*rockhopper.Migration{}

func MergeMigrationsMap(ms map[rockhopper.RegistryKey]*rockhopper.Migration) {
	for k, m := range ms {
		if _, ok := registeredGoMigrations[k] ; !ok {
			registeredGoMigrations[k] = m
		} else {
			log.Printf("the migration key %+v is duplicated: %+v", k, m)
		}
	}
}

func GetMigrationsMap() map[rockhopper.RegistryKey]*rockhopper.Migration {
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

// AddMigration adds a migration with its runtime caller information
func AddMigration(packageName string, up, down rockhopper.TransactionHandler) {
	pc, filename, _, _ := runtime.Caller(1)

	if packageName == "" {
		funcName := runtime.FuncForPC(pc).Name()
		packageName = _parseFuncPackageName(funcName)
	}

	AddNamedMigration(packageName, filename, up, down)
}

// parseFuncPackageName parses the package name from a given runtime caller function name 
func _parseFuncPackageName(funcName string) string {
	lastSlash := strings.LastIndexByte(funcName, '/')
	if lastSlash < 0 {
		lastSlash = 0
	}

	lastDot := strings.LastIndexByte(funcName[lastSlash:], '.') + lastSlash
	packageName := funcName[:lastDot]
	return packageName
}


// AddNamedMigration adds a named migration to the registered go migration map
func AddNamedMigration(packageName, filename string, up, down rockhopper.TransactionHandler) {
	v, err := rockhopper.FileNumericComponent(filename)
	if err != nil {
		panic(fmt.Errorf("unable to parse numeric component from filename %s: %v", filename, err))
	}

	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: v,
		UpFn:    up,
		DownFn:  down,
		Source:  filename,
		UseTx:   true,
	}

	key := rockhopper.RegistryKey{ Package: packageName, Version: v}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with key %+v: %+v", filename, key, existing))
	}

	registeredGoMigrations[key] = migration
}

// AddStatementMigration registers a migration that was compiled from a .sql file.
// The SQL statements are kept as data (rather than baked into a function body) so
// the console can preview each statement while the migration runs.
func AddStatementMigration(packageName string, version int64, source string, useTx bool, upStatements, downStatements []rockhopper.Statement) {
	migration := &rockhopper.Migration{
		Package:    packageName,
		Registered: true,

		Version: version,
		Source:  source,
		UseTx:   useTx,

		UpStatements:   upStatements,
		DownStatements: downStatements,
	}

	key := rockhopper.RegistryKey{ Package: packageName, Version: version}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with key %+v: %+v", source, key, existing))
	}

	registeredGoMigrations[key] = migration
}`))

var migrationTemplate = template.Must(template.New("cmd.go-migration").Funcs(templateFuncs).Parse(`package {{.PackageName}}

import (
	"github.com/c9s/rockhopper/v2"
)

// This migration was compiled from {{ .Migration.Source }}.
// The SQL statements are registered as data so they can be previewed in the
// console while the migration runs, exactly like a raw .sql migration.
func init() {
	AddStatementMigration({{ .Migration.Package | quote }}, {{ .Migration.Version }}, {{ .Migration.Source | quote }}, {{ .Migration.UseTx }},
		[]rockhopper.Statement{
{{- range .Migration.UpStatements }}
			{Direction: rockhopper.DirectionUp, SQL: {{ .SQL | quote }}},
{{- end }}
		},
		[]rockhopper.Statement{
{{- range .Migration.DownStatements }}
			{Direction: rockhopper.DirectionDown, SQL: {{ .SQL | quote }}},
{{- end }}
		},
	)
}`))

type apiTemplateArgs struct {
	PackageName string
}

func renderTemplateAndGoFormatToFile(fp string, tpl *template.Template, a interface{}) error {
	out, err := renderTemplateAndGoFormat(tpl, a)
	if err != nil {
		return err
	}

	return os.WriteFile(fp, out, 0600)
}

func renderTemplateAndGoFormat(tpl *template.Template, a interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := tpl.Execute(buf, a)
	if err != nil {
		return nil, err
	}

	return format.Source(buf.Bytes())
}

type migrationTemplateArgs struct {
	// Migration is the Migration metadata object
	Migration *Migration

	// PackageName is the package name that will be used to render the go file.
	PackageName string
}

var specialCharsRegExp = regexp.MustCompile(`\W`)

func renderMigration(packageName string, m *Migration) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := migrationTemplate.Execute(buf, migrationTemplateArgs{
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

	Wipe bool
}

func (d *GoMigrationDumper) DumpApi() error {
	packageName := d.PackageName
	if len(packageName) == 0 {
		packageName = filepath.Base(d.Dir)
	}

	err := renderTemplateAndGoFormatToFile(filepath.Join(d.Dir, "migration_api.go"), apiTemplate, apiTemplateArgs{
		PackageName: packageName,
	})

	if err != nil {
		return err
	}

	err = renderTemplateAndGoFormatToFile(filepath.Join(d.Dir, "migration_api_test.go"), testTemplate, apiTemplateArgs{
		PackageName: packageName,
	})

	if err != nil {
		return err
	}

	return nil
}

func (d *GoMigrationDumper) Dump(migrations MigrationSlice) error {
	if d.Wipe {
		if err := os.RemoveAll(d.Dir); err != nil {
			return err
		}

		if err := os.MkdirAll(d.Dir, 0755); err != nil {
			return err
		}
	}

	if err := d.DumpApi(); err != nil {
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

	goFilename := filepath.Join(d.Dir,
		specialCharsRegExp.ReplaceAllLiteralString(m.Package, "_")+"_"+replaceExt(filepath.Base(m.Source), ".go"))
	return os.WriteFile(goFilename, out, 0600)
}
