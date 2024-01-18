package rockhopper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileNumericComponent(t *testing.T) {

	var testCases = []struct {
		Filename    string
		WantVersion int64
	}{
		{
			Filename:    "pkg/testing/migrations/app1_20240116231445_create_table_1.go",
			WantVersion: 20240116231445,
		},
	}

	for _, testCase := range testCases {
		version, err := FileNumericComponent(testCase.Filename)
		assert.NoError(t, err)
		assert.Equal(t, testCase.WantVersion, version)
	}
}

func TestSqlMigrationLoader_Load(t *testing.T) {
	loader := &SqlMigrationLoader{}
	migrations, err := loader.Load("testdata/migrations")
	assert.NoError(t, err)
	assert.NotEmpty(t, migrations)
}

func Test_toCamelCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "from_snake_case",
			input: "fix_my_trade",
			want:  "fixMyTrade",
		},
		{
			name:  "with_space",
			input: "fix my trade",
			want:  "fixMyTrade",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toCamelCase(tt.input); got != tt.want {
				t.Errorf("toCamelCase() = %v, want %v", got, tt.want)
			}
		})
	}
}
