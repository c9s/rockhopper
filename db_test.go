package rockhopper

import (
	"testing"

	"github.com/stretchr/testify/assert"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpen(t *testing.T) {
	dialect, err := LoadDialect("sqlite3")
	assert.NoError(t, err)

	db, err := Open("sqlite3", dialect, ":memory:", legacyGooseTableName)
	assert.NoError(t, err)
	assert.NotNil(t, db)

	err = db.Close()
	assert.NoError(t, err)
}
