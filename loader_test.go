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

func Test_toCamelCase(t *testing.T) {
	tests := []struct {
		name string
		input string
		want string
	}{
		{
			name: "from_snake_case",
			input: "fix_my_trade",
			want: "fixMyTrade",
		},
		{
			name: "with_space",
			input: "fix my trade",
			want: "fixMyTrade",
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
