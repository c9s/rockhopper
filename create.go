package rockhopper

import (
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const timestampFormat = "20060102150405"

// Create writes a new blank migration file.
func CreateWithTemplate(dir string, tmpl *template.Template, name, migrationType string) error {
	type tmplVars struct {
		Version   string
		CamelName string
	}

	version := time.Now().Format(timestampFormat)
	filename := fmt.Sprintf("%v_%v.%v", version, snakeCase(name), migrationType)

	if tmpl == nil {
		if migrationType == "go" {
			tmpl = goSQLMigrationTemplate
		} else {
			tmpl = sqlMigrationTemplate
		}
	}

	path := filepath.Join(dir, filename)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		return errors.Wrap(err, "failed to create migration file")
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create migration file")
	}
	defer f.Close()

	vars := tmplVars{
		Version:   version,
		CamelName: toCamelCase(name),
	}
	if err := tmpl.Execute(f, vars); err != nil {
		return errors.Wrap(err, "failed to execute tmpl")
	}

	log.Printf("created new migration file: %s", f.Name())
	return nil
}

var sqlMigrationTemplate = template.Must(template.New("goose.sql-migration").Parse(`-- +up
-- +begin
SELECT 'up SQL query';
-- +end

-- +down

-- +begin
SELECT 'down SQL query';
-- +end
`))

var goSQLMigrationTemplate = template.Must(template.New("goose.go-migration").Parse(`package migrations

import (
	"database/sql"
	"github.com/c9s/rockhopper"
)

func init() {
	rockhopper.AddMigration(up{{.CamelName}}, down{{.CamelName}})
}

func up{{.CamelName}}(tx *sql.Tx) error {
	// This code is executed when the migration is applied.
	return nil
}

func down{{.CamelName}}(tx *sql.Tx) error {
	// This code is executed when the migration is rolled back.
	return nil
}
`))
