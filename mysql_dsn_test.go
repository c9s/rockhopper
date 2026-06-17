package rockhopper

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// clearMySQLEnv removes any MYSQL_ / MYSQL8_ variables from the test process so
// each case starts from a clean slate.
func clearMySQLEnv(t *testing.T) {
	t.Helper()
	for _, env := range os.Environ() {
		key := env[:strings.IndexByte(env, '=')]
		if strings.HasPrefix(key, "MYSQL_") || strings.HasPrefix(key, "MYSQL8_") {
			t.Setenv(key, "") // ensure t.Setenv records it for restore
			os.Unsetenv(key)
		}
	}
}

func TestBuildMySqlDSN_UnixSocket(t *testing.T) {
	clearMySQLEnv(t)

	t.Setenv("MYSQL_USER", "root")
	t.Setenv("MYSQL_PASSWORD", "123123")
	t.Setenv("MYSQL_UNIX_PORT", "/opt/local/var/run/mysql8/mysqld.sock")
	t.Setenv("MYSQL_DATABASE", "test")

	dsn, err := buildMySqlDSN()
	if assert.NoError(t, err) {
		assert.Equal(t, "root:123123@unix(/opt/local/var/run/mysql8/mysqld.sock)/test", dsn)
	}
}

func TestBuildMySqlDSN_UnixSocketTakesPrecedenceOverHost(t *testing.T) {
	clearMySQLEnv(t)

	t.Setenv("MYSQL_USER", "root")
	t.Setenv("MYSQL_HOST", "127.0.0.1")
	t.Setenv("MYSQL_PORT", "3306")
	t.Setenv("MYSQL_UNIX_PORT", "/tmp/mysql.sock")

	dsn, err := buildMySqlDSN()
	if assert.NoError(t, err) {
		assert.Equal(t, "root@unix(/tmp/mysql.sock)/", dsn)
	}
}

func TestBuildMySqlDSN_TCPWhenNoSocket(t *testing.T) {
	clearMySQLEnv(t)

	t.Setenv("MYSQL_USER", "root")
	t.Setenv("MYSQL_HOST", "127.0.0.1")
	t.Setenv("MYSQL_PORT", "3306")
	t.Setenv("MYSQL_DATABASE", "test")

	dsn, err := buildMySqlDSN()
	if assert.NoError(t, err) {
		assert.Equal(t, "root@tcp(127.0.0.1:3306)/test", dsn)
	}
}

func TestBuildMySqlDSN_MySQL8PrefixUnixSocket(t *testing.T) {
	clearMySQLEnv(t)

	// Presence of any MYSQL8_ variable switches the whole lookup to the
	// MYSQL8_ prefix, including MYSQL8_UNIX_PORT.
	t.Setenv("MYSQL8_USER", "root")
	t.Setenv("MYSQL8_UNIX_PORT", "/opt/local/var/run/mysql8/mysqld.sock")
	t.Setenv("MYSQL8_DATABASE", "test")

	dsn, err := buildMySqlDSN()
	if assert.NoError(t, err) {
		assert.Equal(t, "root@unix(/opt/local/var/run/mysql8/mysqld.sock)/test", dsn)
	}
}

func TestBuildMySqlDSN_URLPassthrough(t *testing.T) {
	clearMySQLEnv(t)

	t.Setenv("MYSQL_URL", "root:123123@unix(/opt/local/var/run/mysql8/mysqld.sock)/test")

	dsn, err := buildMySqlDSN()
	if assert.NoError(t, err) {
		assert.Equal(t, "root:123123@unix(/opt/local/var/run/mysql8/mysqld.sock)/test", dsn)
	}
}
