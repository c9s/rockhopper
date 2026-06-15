package rockhopper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillNames(t *testing.T) {
	names, err := SkillNames()
	require.NoError(t, err)
	assert.NotEmpty(t, names, "expected at least one bundled skill")
}

func TestInstallSkills(t *testing.T) {
	dir := t.TempDir()

	written, skipped, err := InstallSkills(dir, false)
	require.NoError(t, err)
	assert.NotEmpty(t, written)
	assert.Empty(t, skipped)

	// every bundled skill should land at .claude/skills/<name>/SKILL.md
	names, err := SkillNames()
	require.NoError(t, err)
	for _, name := range names {
		p := filepath.Join(dir, ".claude", "skills", name, "SKILL.md")
		_, statErr := os.Stat(p)
		assert.NoError(t, statErr, "expected installed skill %q at %s", name, p)
	}

	// a second run without force skips everything, writing nothing new
	written2, skipped2, err := InstallSkills(dir, false)
	require.NoError(t, err)
	assert.Empty(t, written2)
	assert.ElementsMatch(t, written, skipped2)

	// with force, the same files are rewritten
	written3, skipped3, err := InstallSkills(dir, true)
	require.NoError(t, err)
	assert.ElementsMatch(t, written, written3)
	assert.Empty(t, skipped3)
}
