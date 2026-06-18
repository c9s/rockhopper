package rockhopper

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestMigrationParser_ParseBytes(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:  "up and down",
			input: "testdata/migrations/20200721225616_trades.sql",
		},
		{
			name:  "statement begin and end",
			input: "testdata/migrations/20200819054742_trade_index.sql",
		},
	}

	type Fixture struct {
		UpStatements   []Statement `json:"up" yaml:"upStmts"`
		DownStatements []Statement `json:"down" yaml:"downStmts"`
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(tt.input)
			assert.NoError(t, err)

			p := &MigrationParser{}
			chunk, err := p.ParseBytes(data)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			gotUpStmts, gotDownStmts := chunk.UpStmts, chunk.DownStmts

			fixtureFile := tt.input + ".fixture"

			if _, err := os.Stat(fixtureFile); os.IsNotExist(err) {
				// write fixture
				fixture := Fixture{
					UpStatements:   gotUpStmts,
					DownStatements: gotDownStmts,
				}

				out, err := yaml.Marshal(fixture)
				assert.NoError(t, err)

				err = os.WriteFile(fixtureFile, out, 0600)
				assert.NoError(t, err)
			} else {
				fixtureData, err := os.ReadFile(fixtureFile)
				assert.NoError(t, err)

				var fixture Fixture
				err = yaml.Unmarshal(fixtureData, &fixture)
				assert.NoError(t, err)

				if !reflect.DeepEqual(gotUpStmts, fixture.UpStatements) {
					t.Errorf("ParseBytes() gotUpStmts = %v, want %v", gotUpStmts, fixture.DownStatements)
				}
				if !reflect.DeepEqual(gotDownStmts, fixture.DownStatements) {
					t.Errorf("ParseBytes() gotDownStmts = %v, want %v", gotDownStmts, fixture.DownStatements)
				}

			}

		})
	}
}

func TestMigrationParser_GooseAnnotations(t *testing.T) {
	p := &MigrationParser{}

	t.Run("up and down", func(t *testing.T) {
		goose := "-- +goose Up\nCREATE TABLE post (id int);\n-- +goose Down\nDROP TABLE post;\n"
		native := "-- +up\nCREATE TABLE post (id int);\n-- +down\nDROP TABLE post;\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		n, err := p.ParseString(native)
		assert.NoError(t, err)
		assert.Equal(t, n, g, "goose Up/Down should parse the same as native +up/+down")
	})

	t.Run("statement begin and end", func(t *testing.T) {
		goose := "-- +goose Up\n-- +goose StatementBegin\nCREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;\n-- +goose StatementEnd\n"
		native := "-- +up\n-- +begin\nCREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;\n-- +end\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		n, err := p.ParseString(native)
		assert.NoError(t, err)

		assert.Equal(t, n, g, "goose StatementBegin/StatementEnd should parse the same as native +begin/+end")
		// the whole multi-statement block must be captured as a single statement
		if assert.Len(t, g.UpStmts, 1) {
			assert.Contains(t, g.UpStmts[0].SQL, "END; $$ LANGUAGE plpgsql;")
		}
	})

	t.Run("no transaction", func(t *testing.T) {
		goose := "-- +goose NO TRANSACTION\n-- +goose Up\nCREATE INDEX CONCURRENTLY idx ON t (c);\n"
		native := "-- !txn\n-- +up\nCREATE INDEX CONCURRENTLY idx ON t (c);\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		n, err := p.ParseString(native)
		assert.NoError(t, err)

		assert.Equal(t, n, g, "goose NO TRANSACTION should parse the same as native !txn")
		assert.False(t, g.UseTx, "NO TRANSACTION should disable transaction wrapping")
	})

	t.Run("full file with statement blocks in both directions", func(t *testing.T) {
		goose := "-- +goose Up\n" +
			"-- +goose StatementBegin\n" +
			"CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;\n" +
			"-- +goose StatementEnd\n" +
			"INSERT INTO t (c) VALUES (1);\n" +
			"-- +goose Down\n" +
			"-- +goose StatementBegin\n" +
			"DROP FUNCTION f();\n" +
			"-- +goose StatementEnd\n" +
			"DELETE FROM t WHERE c = 1;\n"
		native := "-- +up\n" +
			"-- +begin\n" +
			"CREATE FUNCTION f() RETURNS int AS $$ BEGIN RETURN 1; END; $$ LANGUAGE plpgsql;\n" +
			"-- +end\n" +
			"INSERT INTO t (c) VALUES (1);\n" +
			"-- +down\n" +
			"-- +begin\n" +
			"DROP FUNCTION f();\n" +
			"-- +end\n" +
			"DELETE FROM t WHERE c = 1;\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		n, err := p.ParseString(native)
		assert.NoError(t, err)

		assert.Equal(t, n, g, "a complete goose file should parse identically to its native equivalent")
		assert.Len(t, g.UpStmts, 2)
		assert.Len(t, g.DownStmts, 2)
	})

	t.Run("annotation keywords are case-insensitive", func(t *testing.T) {
		// goose itself is case-sensitive, but we are lenient so that
		// hand-written variants still translate correctly.
		goose := "-- +goose up\nCREATE TABLE post (id int);\n-- +goose DOWN\nDROP TABLE post;\n"
		native := "-- +up\nCREATE TABLE post (id int);\n-- +down\nDROP TABLE post;\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		n, err := p.ParseString(native)
		assert.NoError(t, err)
		assert.Equal(t, n, g, "goose keyword casing should not affect parsing")
	})

	t.Run("package annotation alongside goose annotations", func(t *testing.T) {
		goose := "-- @package mypkg\n-- +goose Up\nCREATE TABLE post (id int);\n-- +goose Down\nDROP TABLE post;\n"

		g, err := p.ParseString(goose)
		assert.NoError(t, err)
		assert.Equal(t, "mypkg", g.Package)
		assert.Len(t, g.UpStmts, 1)
		assert.Len(t, g.DownStmts, 1)
	})
}

func Test_matchPackageName(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		pkgName, err := matchPackageName("@package main")
		if assert.NoError(t, err) {
			assert.Equal(t, "main", pkgName)
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		pkgName, err := matchPackageName("-- @package main")
		if assert.NoError(t, err) {
			assert.Equal(t, "main", pkgName)
		}
	})

	t.Run("go package name", func(t *testing.T) {
		pkgName, err := matchPackageName("-- @package github.com/c9s/bbgo")
		if assert.NoError(t, err) {
			assert.Equal(t, "github.com/c9s/bbgo", pkgName)
		}
	})
}
