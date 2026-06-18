//go:build !no_mysql
// +build !no_mysql

package rockhopper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeMySQLDSN_ParseTime guards that the DSN normalizer registered by
// the MySQL driver always yields parseTime=true, which rockhopper needs so the
// version table's tstamp column scans into time.Time.
func TestNormalizeMySQLDSN_ParseTime(t *testing.T) {
	require.NotNil(t, normalizeMySQLDSN, "the MySQL driver must register the DSN normalizer")

	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "adds parseTime when missing",
			dsn:  "root:pass@tcp(127.0.0.1:3306)/test",
			want: "root:pass@tcp(127.0.0.1:3306)/test?parseTime=true",
		},
		{
			name: "keeps parseTime when already set",
			dsn:  "root:pass@tcp(127.0.0.1:3306)/test?parseTime=true",
			want: "root:pass@tcp(127.0.0.1:3306)/test?parseTime=true",
		},
		{
			name: "appends parseTime alongside other params",
			dsn:  "root:pass@tcp(127.0.0.1:3306)/test?charset=utf8mb4",
			want: "root:pass@tcp(127.0.0.1:3306)/test?charset=utf8mb4&parseTime=true",
		},
		{
			name: "preserves unix socket",
			dsn:  "root:pass@unix(/tmp/mysql.sock)/test",
			want: "root:pass@unix(/tmp/mysql.sock)/test?parseTime=true",
		},
		{
			name: "no database name",
			dsn:  "root@tcp(127.0.0.1:3306)/",
			want: "root@tcp(127.0.0.1:3306)/?parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeMySQLDSN(tt.dsn)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeMySQLDSN_Invalid(t *testing.T) {
	require.NotNil(t, normalizeMySQLDSN)

	_, err := normalizeMySQLDSN("::::not a dsn")
	assert.Error(t, err)
}
