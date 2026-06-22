package rockhopper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "rockhopper.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestLoadConfig_MigratesLegacyMigrationsDir(t *testing.T) {
	t.Run("legacy-only config becomes a single-element list", func(t *testing.T) {
		path := writeTempConfig(t, "driver: sqlite3\nmigrationsDir: migrations/sqlite3\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, []string{"migrations/sqlite3"}, config.MigrationsDirs)
		}
	})

	t.Run("legacy dir is prepended so it stays first", func(t *testing.T) {
		path := writeTempConfig(t, "driver: mysql\nmigrationsDir: legacy/dir\nmigrationsDirs:\n- modern/a\n- modern/b\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, []string{"legacy/dir", "modern/a", "modern/b"}, config.MigrationsDirs)
		}
	})

	t.Run("migrationsDirs only is left untouched", func(t *testing.T) {
		path := writeTempConfig(t, "driver: mysql\nmigrationsDirs:\n- a\n- b\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, []string{"a", "b"}, config.MigrationsDirs)
		}
	})

	t.Run("no directory configured falls back to migrations", func(t *testing.T) {
		path := writeTempConfig(t, "driver: sqlite3\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, []string{"migrations"}, config.MigrationsDirs)
		}
	})
}

func TestLoadConfig_ExpandsEnv(t *testing.T) {
	t.Run("braced form", func(t *testing.T) {
		t.Setenv("MYSQL8_URL", "root@tcp(localhost:3306)/db?parseTime=true")

		path := writeTempConfig(t, "driver: mysql\ndialect: mysql\ndsn: ${MYSQL8_URL}\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, "root@tcp(localhost:3306)/db?parseTime=true", config.DSN)
		}
	})

	t.Run("bare form", func(t *testing.T) {
		t.Setenv("MYSQL8_URL", "user:pass@tcp(db:3306)/app")

		path := writeTempConfig(t, "driver: mysql\ndsn: $MYSQL8_URL\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, "user:pass@tcp(db:3306)/app", config.DSN)
		}
	})

	t.Run("undefined expands to empty", func(t *testing.T) {
		os.Unsetenv("ROCKHOPPER_NOPE")

		path := writeTempConfig(t, "driver: mysql\ndsn: ${ROCKHOPPER_NOPE}\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, "", config.DSN)
		}
	})

	t.Run("dollar escape", func(t *testing.T) {
		path := writeTempConfig(t, "driver: mysql\ndsn: \"user:p@$$w0rd@tcp(db:3306)/app\"\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			assert.Equal(t, "user:p@$w0rd@tcp(db:3306)/app", config.DSN)
		}
	})

	t.Run("env var override still wins", func(t *testing.T) {
		t.Setenv("MYSQL8_URL", "from-file")
		t.Setenv("ROCKHOPPER_DSN", "from-rockhopper-env")

		path := writeTempConfig(t, "driver: mysql\ndsn: ${MYSQL8_URL}\n")
		config, err := LoadConfig(path)
		if assert.NoError(t, err) {
			// ROCKHOPPER_DSN (via the env: struct tag) overrides the file value.
			assert.Equal(t, "from-rockhopper-env", config.DSN)
		}
	})
}
