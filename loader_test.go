package rockhopper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSqlMigrationLoader_Load(t *testing.T) {
	loader := &SqlMigrationLoader{}
	migrations, err := loader.Load("testdata/migrations")
	assert.NoError(t, err)
	assert.NotEmpty(t, migrations)
}
