package rockhopper

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

// skillsFS embeds the bundled Claude Code skills so they can be scaffolded into
// downstream projects via `rockhopper skills install`. The skills are versioned
// with the binary, which keeps them in sync with the CLI flags they invoke.
//
//go:embed all:.claude/skills
var skillsFS embed.FS

// skillsRoot is the path of the embedded skills within skillsFS.
const skillsRoot = ".claude/skills"

// SkillNames returns the names of the Claude Code skills bundled in the binary.
func SkillNames() ([]string, error) {
	entries, err := fs.ReadDir(skillsFS, skillsRoot)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}

	return names, nil
}

// InstallSkills writes the bundled Claude Code skills into <dir>/.claude/skills,
// preserving the directory layout. Existing files are left untouched unless force
// is true. It returns the destination paths that were written and the ones skipped.
func InstallSkills(dir string, force bool) (written, skipped []string, err error) {
	err = fs.WalkDir(skillsFS, skillsRoot, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			return nil
		}

		data, readErr := skillsFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}

		// p looks like ".claude/skills/<name>/SKILL.md"; mirror that layout under dir.
		dest := filepath.Join(dir, filepath.FromSlash(p))

		if !force {
			if _, statErr := os.Stat(dest); statErr == nil {
				skipped = append(skipped, dest)
				return nil
			}
		}

		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return mkErr
		}

		if writeErr := os.WriteFile(dest, data, 0o644); writeErr != nil {
			return writeErr
		}

		written = append(written, dest)
		return nil
	})

	return written, skipped, err
}
