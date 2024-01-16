package rockhopper

import (
	"io/ioutil"
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
			data, err := ioutil.ReadFile(tt.input)
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

				err = ioutil.WriteFile(fixtureFile, out, 0666)
				assert.NoError(t, err)
			} else {
				fixtureData, err := ioutil.ReadFile(fixtureFile)
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
