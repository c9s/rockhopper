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
	MergeMigrationsMap(map[registryKey]*rockhopper.Migration{
		registryKey{ Version: 2 }: &rockhopper.Migration{},
		registryKey{ Version: 2 }: &rockhopper.Migration{},
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

type registryKey struct {
	Package string
	Version int64
}

var registeredGoMigrations = map[registryKey]*rockhopper.Migration{}

func MergeMigrationsMap(ms map[registryKey]*rockhopper.Migration) {
	for k, m := range ms {
		if _, ok := registeredGoMigrations[k] ; !ok {
			registeredGoMigrations[k] = m
		} else {
			log.Printf("the migration key %+v is duplicated: %+v", k, m)
		}
	}
}

func GetMigrationsMap() map[registryKey]*rockhopper.Migration {
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
	if registeredGoMigrations == nil {
		registeredGoMigrations = make(map[registryKey]*rockhopper.Migration)
	}

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

	key := registryKey{ Package: packageName, Version: v}
	if existing, ok := registeredGoMigrations[key]; ok {
		panic(fmt.Sprintf("failed to add migration %q: version conflicts with key %+v: %+v", filename, key, existing))
	}

	registeredGoMigrations[key] = migration
}`))

var migrationTemplate = template.Must(template.New("cmd.go-migration").Funcs(templateFuncs).Parse(`package {{.PackageName}}

import (
	"context"

	"github.com/c9s/rockhopper/v2"
)

func init() {
	AddMigration({{ .Migration.Package | quote }}, up{{ .FuncNameBody }}, down{{ .FuncNameBody }})

{{ if .Global }}
	rockhopper.AddMigration(up{{.FuncNameBody}}, down{{.FuncNameBody}})
{{ end }}
}

func up{{ .FuncNameBody }}(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is applied.
{{- range .Migration.UpStatements }}
	_, err = tx.ExecContext(ctx, {{ .SQL | quote }})
	if err != nil {
		return err
	}

{{- end }}
	return err
}

func down{{ .FuncNameBody }}(ctx context.Context, tx rockhopper.SQLExecutor) (err error) {
	// This code is executed when the migration is rolled back.
{{- range .Migration.DownStatements }}
	_, err = tx.ExecContext(ctx, {{ .SQL | quote }})
	if err != nil {
		return err
	}

{{- end }}
	return err
}`))

type apiTemplateArgs struct {
	PackageName string
}

func renderTemplateAndGoFormatToFile(fp string, tpl *template.Template, a interface{}) error {
	out, err := renderTemplateAndGoFormat(tpl, a)
	if err != nil {
		return err
	}

	return os.WriteFile(fp, out, 0666)
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
	FuncNameBody string

	CamelName string

	// BaseName is the file basename of the migration script.
	BaseName string

	// Migration is the Migration metadata object
	Migration *Migration

	// PackageName is the package name that will be used to render the go file.
	PackageName string

	// Global renders the migration template with the global migration registration calls.
	// This parameter avoids migration version conflict for different sql dialect (or driver)
	Global bool
}

var specialCharsRegExp = regexp.MustCompile("\\W")

func renderMigration(packageName string, m *Migration) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	funcNameBody := "_" + specialCharsRegExp.ReplaceAllLiteralString(m.Package+"_"+toCamelCase(m.Name), "_")
	err := migrationTemplate.Execute(buf, migrationTemplateArgs{
		FuncNameBody: funcNameBody,
		CamelName:    strings.ToTitle(toCamelCase(m.Name)),
		BaseName:     filepath.Base(m.Source),
		Migration:    m,
		PackageName:  packageName,
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
	return os.WriteFile(goFilename, out, 0666)
}
