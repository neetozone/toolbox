package files

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func writeLockFiles(t *testing.T, dir string, contents map[string]string) {
	for name, content := range contents {
		assert.Nil(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}
}

func checksumOf(t *testing.T, path string) string {
	checksum, err := GenerateChecksum(path)
	assert.Nil(t, err)
	return checksum
}

func Test__EntryTypes(t *testing.T) {
	assert.Equal(t, []string{
		"bootsnap", "composer", "gems", "go", "maven", "maven-target", "mix-build",
		"mix-deps", "node-modules", "nvm", "pods", "requirements",
	}, EntryTypes())
}

func Test__LockFileTypesMatchLookupEntries(t *testing.T) {
	dir := t.TempDir()
	for _, lockFile := range lockFiles {
		writeLockFiles(t, dir, map[string]string{lockFile: "x"})
		result := resultForfile(filepath.Join(dir, lockFile), "", LookupOptions{GitBranch: "b"})
		assert.NotNil(t, result, lockFile)
		assert.Len(t, result.Entries, len(lockFileTypes[lockFile]), lockFile)
		for index, entry := range result.Entries {
			assert.Contains(t, entry.Keys[0], "b-"+lockFileTypes[lockFile][index]+"-", lockFile)
		}
	}
}

func Test__ParseEntryTypes(t *testing.T) {
	types, err := ParseEntryTypes("gems,nvm,bootsnap")
	assert.Nil(t, err)
	assert.Equal(t, []string{"gems", "nvm", "bootsnap"}, types)

	types, err = ParseEntryTypes(" gems , gems,,node-modules ")
	assert.Nil(t, err)
	assert.Equal(t, []string{"gems", "node-modules"}, types)

	_, err = ParseEntryTypes("gems,foo")
	assert.Contains(t, fmt.Sprint(err), "unknown cache type 'foo'")

	_, err = ParseEntryTypes("gems-ruby3.3.5-arm64")
	assert.Contains(t, fmt.Sprint(err), "unknown cache type")

	_, err = ParseEntryTypes(" , ")
	assert.Contains(t, fmt.Sprint(err), "at least one type")
}

func Test__LookupTypes(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	writeLockFiles(t, dir, map[string]string{
		"Gemfile.lock":      "GEM\n\nRUBY VERSION\n   ruby 3.3.5p100\n",
		"yarn.lock":         "# yarn\n",
		"package-lock.json": "{}\n",
		".nvmrc":            "20\n",
		"go.sum":            "x\n",
	})

	gems := checksumOf(t, filepath.Join(dir, "Gemfile.lock"))
	yarn := checksumOf(t, filepath.Join(dir, "yarn.lock"))
	nvm := checksumOf(t, filepath.Join(dir, ".nvmrc"))
	gemsPrefix := fmt.Sprintf("gems-ruby3.3.5-%s", runtime.GOARCH)

	t.Run("store keeps only the selected types", func(t *testing.T) {
		options := LookupOptions{LookupDirectory: dir, HomeDirectory: home, GitBranch: "main", DefaultBranch: "main"}
		results, missing := LookupTypes(options, []string{"gems", "nvm", "bootsnap", "node-modules"})
		assert.Empty(t, missing)
		assertLookupResults(t, results, []LookupResult{
			{DetectedFile: ".nvmrc", Entries: []LookupResultEntry{
				{Path: filepath.Join(home, ".nvm"), Keys: []string{"main-nvm-" + nvm}, EquivalentKeys: []string{}},
			}},
			{DetectedFile: "Gemfile.lock", Entries: []LookupResultEntry{
				{Path: filepath.FromSlash("vendor/bundle"), Keys: []string{fmt.Sprintf("main-%s-%s", gemsPrefix, gems)}, EquivalentKeys: []string{}},
				{Path: filepath.FromSlash("tmp/bootsnap-cache"), Keys: []string{"main-bootsnap-" + gems}, EquivalentKeys: []string{}},
			}},
			{DetectedFile: "yarn.lock", Entries: []LookupResultEntry{
				{Path: "node_modules", Keys: []string{"main-node-modules-" + yarn}, EquivalentKeys: []string{}},
			}},
		})
	})

	t.Run("restore keys match automatic mode", func(t *testing.T) {
		options := LookupOptions{LookupDirectory: dir, HomeDirectory: home, GitBranch: "feature", DefaultBranch: "main", Restore: true}
		results, missing := LookupTypes(options, []string{"gems"})
		assert.Empty(t, missing)
		assert.Len(t, results, 1)

		automatic := Lookup(options)
		for _, result := range automatic {
			if result.DetectedFile == "Gemfile.lock" {
				assert.Equal(t, result.Entries, results[0].Entries)
			}
		}

		assert.Equal(t, []string{
			fmt.Sprintf("feature-%s-%s", gemsPrefix, gems),
			fmt.Sprintf("main-%s-%s", gemsPrefix, gems),
			"feature-" + gemsPrefix,
			"main-" + gemsPrefix,
		}, results[0].Entries[0].Keys)
	})

	t.Run("types without a lock file are reported as missing", func(t *testing.T) {
		results, missing := LookupTypes(LookupOptions{LookupDirectory: dir, GitBranch: "main"}, []string{"pods", "gems", "mix-deps"})
		assert.Equal(t, []string{"pods", "mix-deps"}, missing)
		assert.Len(t, results, 1)
		assert.Equal(t, "Gemfile.lock", results[0].DetectedFile)
	})

	t.Run("an empty directory has nothing to do", func(t *testing.T) {
		results, missing := LookupTypes(LookupOptions{LookupDirectory: t.TempDir(), GitBranch: "main"}, []string{"gems", "bootsnap"})
		assert.Empty(t, results)
		assert.Equal(t, []string{"gems", "bootsnap"}, missing)
	})

	t.Run("nested directories get scoped keys except absolute paths", func(t *testing.T) {
		nested := filepath.Join(dir, "sub")
		assert.Nil(t, os.MkdirAll(nested, 0700))
		writeLockFiles(t, nested, map[string]string{"Gemfile.lock": "GEM\n", ".nvmrc": "20\n"})
		options := LookupOptions{LookupDirectory: nested, RootDirectory: dir, HomeDirectory: home, GitBranch: "main"}
		results, _ := LookupTypes(options, []string{"gems", "bootsnap", "nvm"})
		keys := []string{}
		for _, result := range results {
			for _, entry := range result.Entries {
				keys = append(keys, entry.Keys[0])
			}
		}

		sub := checksumOf(t, filepath.Join(nested, "Gemfile.lock"))
		assert.Equal(t, []string{
			"main-nvm-" + checksumOf(t, filepath.Join(nested, ".nvmrc")),
			fmt.Sprintf("main-gems-%s@sub-%s", runtime.GOARCH, sub),
			"main-bootsnap@sub-" + sub,
		}, keys)
	})
}
