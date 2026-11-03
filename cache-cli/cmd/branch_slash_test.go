package cmd

import (
	"os"
	"path/filepath"
	"testing"

	files "github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	assert "github.com/stretchr/testify/assert"
)

func Test__BranchesWithSlashesMatchStoredKeys(t *testing.T) {
	dir := t.TempDir()
	assert.Nil(t, os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte("lock"), 0600))

	lookup := func(branch string, restore bool) files.LookupResultEntry {
		results := files.Lookup(files.LookupOptions{
			LookupDirectory: dir,
			RootDirectory:   dir,
			GitBranch:       branch,
			DefaultBranch:   "release/stable",
			Restore:         restore,
		})
		assert.Len(t, results, 1)
		return results[0].Entries[0]
	}

	normalized := func(keys []string) []string {
		out := []string{}
		for _, key := range keys {
			out = append(out, NormalizeKey(key))
		}
		return out
	}

	t.Run("fresh branch restores the default-branch archive", func(t *testing.T) {
		stored := normalized(lookup("release/stable", false).Keys)
		assert.Len(t, stored, 1)
		assert.Regexp(t, `^release-stable-node-modules-[0-9a-f]{32}$`, stored[0])

		restoreKeys := normalized(lookup("feature/foo", true).Keys)
		assert.Contains(t, restoreKeys, stored[0])
		assert.Contains(t, restoreKeys, "release-stable-node-modules")
	})

	t.Run("a slash in the branch matches the key stored for it", func(t *testing.T) {
		stored := normalized(lookup("feature/foo", false).Keys)
		assert.Len(t, stored, 1)
		assert.Regexp(t, `^feature-foo-node-modules-[0-9a-f]{32}$`, stored[0])

		restoreKeys := normalized(lookup("feature/foo", true).Keys)
		assert.Equal(t, stored[0], restoreKeys[0])
	})
}
