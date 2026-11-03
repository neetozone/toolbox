package files

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func Test__LookupStore(t *testing.T) {
	homedir, _ := os.UserHomeDir()

	// TODO: find a better way to find the root path
	_, b, _, _ := runtime.Caller(0)
	testFilePath := filepath.Dir(b)
	pkgPath := filepath.Dir(testFilePath)
	rootPath := filepath.Dir(pkgPath)

	t.Run("finds nothing", func(t *testing.T) {
		results := Lookup(LookupOptions{Restore: false, LookupDirectory: testFilePath})
		assert.Empty(t, results)
	})

	t.Run("finds .nvmrc", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/nvm/.nvmrc", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/nvm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: ".nvmrc",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/.nvm", homedir)),
						Keys: []string{fmt.Sprintf("master-nvm-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds Gemfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/gems/Gemfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/gems", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Gemfile.lock",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash("vendor/bundle"),
						Keys: []string{fmt.Sprintf("master-gems-%s-%s", runtime.GOARCH, checksum)},
					},
				},
			},
		})
	})

	t.Run("finds package-lock.json", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("master-node-modules-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds requirements.txt", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/pip/requirements.txt", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/pip", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{fmt.Sprintf("master-requirements-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds composer.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/composer/composer.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/composer", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "composer.lock",
				Entries: []LookupResultEntry{
					{Path: "vendor", Keys: []string{fmt.Sprintf("master-composer-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds Podfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/cocoapods/Podfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/cocoapods", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Podfile.lock",
				Entries: []LookupResultEntry{
					{Path: "Pods", Keys: []string{fmt.Sprintf("master-pods-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds go.sum", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/go/go.sum", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/go", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "go.sum",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/go/pkg/mod", homedir)),
						Keys: []string{fmt.Sprintf("master-go-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds yarn.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{
						Path: "node_modules",
						Keys: []string{fmt.Sprintf("master-node-modules-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds mix.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/elixir/mix.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/elixir", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "mix.lock",
				Entries: []LookupResultEntry{
					{Path: "deps", Keys: []string{fmt.Sprintf("master-mix-deps-%s", checksum)}},
					{Path: "_build", Keys: []string{fmt.Sprintf("master-mix-build-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds pom.xml", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/maven/pom.xml", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/maven", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "pom.xml",
				Entries: []LookupResultEntry{
					{Path: ".m2", Keys: []string{fmt.Sprintf("master-maven-%s", checksum)}},
					{Path: "target", Keys: []string{fmt.Sprintf("master-maven-target-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds requirements.txt and package-lock.json", func(t *testing.T) {
		requirementsChecksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/multiple-files/requirements.txt", rootPath))
		assert.Nil(t, err)

		packageLockChecksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/multiple-files/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/multiple-files", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("master-node-modules-%s", packageLockChecksum)}},
				},
			},
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{fmt.Sprintf("master-requirements-%s", requirementsChecksum)}},
				},
			},
		})
	})
}

func Test__LookupRestore(t *testing.T) {
	homedir, _ := os.UserHomeDir()

	// TODO: find a better way to find the root path
	_, b, _, _ := runtime.Caller(0)
	testFilePath := filepath.Dir(b)
	pkgPath := filepath.Dir(testFilePath)
	rootPath := filepath.Dir(pkgPath)

	t.Run("finds nothing", func(t *testing.T) {
		results := Lookup(LookupOptions{Restore: true, LookupDirectory: testFilePath})
		assert.Empty(t, results)
	})

	t.Run("finds .nvmrc", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/nvm/.nvmrc", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/nvm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: ".nvmrc",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/.nvm", homedir)),
						Keys: []string{
							fmt.Sprintf("some-branch-nvm-%s", checksum),
							fmt.Sprintf("master-nvm-%s", checksum),
							fmt.Sprintf("main-nvm-%s", checksum),
							"some-branch-nvm",
							"master-nvm",
							"main-nvm",
						},
					},
				},
			},
		})
	})

	t.Run("finds Gemfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/gems/Gemfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/gems", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Gemfile.lock",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash("vendor/bundle"),
						Keys: []string{
							fmt.Sprintf("some-branch-gems-%s-%s", runtime.GOARCH, checksum),
							fmt.Sprintf("master-gems-%s-%s", runtime.GOARCH, checksum),
							fmt.Sprintf("main-gems-%s-%s", runtime.GOARCH, checksum),
							fmt.Sprintf("some-branch-gems-%s", runtime.GOARCH),
							fmt.Sprintf("master-gems-%s", runtime.GOARCH),
							fmt.Sprintf("main-gems-%s", runtime.GOARCH),
						},
					},
				},
			},
		})
	})

	t.Run("finds package-lock.json", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("some-branch-node-modules-%s", checksum),
						fmt.Sprintf("master-node-modules-%s", checksum),
						fmt.Sprintf("main-node-modules-%s", checksum),
						"some-branch-node-modules",
						"master-node-modules",
						"main-node-modules",
					}},
				},
			},
		})
	})

	t.Run("finds requirements.txt", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/pip/requirements.txt", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/pip", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{
						fmt.Sprintf("some-branch-requirements-%s", checksum),
						fmt.Sprintf("master-requirements-%s", checksum),
						fmt.Sprintf("main-requirements-%s", checksum),
						"some-branch-requirements",
						"master-requirements",
						"main-requirements",
					}},
				},
			},
		})
	})

	t.Run("finds composer.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/composer/composer.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/composer", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "composer.lock",
				Entries: []LookupResultEntry{
					{Path: "vendor", Keys: []string{
						fmt.Sprintf("some-branch-composer-%s", checksum),
						fmt.Sprintf("master-composer-%s", checksum),
						fmt.Sprintf("main-composer-%s", checksum),
						"some-branch-composer",
						"master-composer",
						"main-composer",
					}},
				},
			},
		})
	})

	t.Run("finds Podfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/cocoapods/Podfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/cocoapods", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Podfile.lock",
				Entries: []LookupResultEntry{
					{Path: "Pods", Keys: []string{
						fmt.Sprintf("some-branch-pods-%s", checksum),
						fmt.Sprintf("master-pods-%s", checksum),
						fmt.Sprintf("main-pods-%s", checksum),
						"some-branch-pods",
						"master-pods",
						"main-pods",
					}},
				},
			},
		})
	})

	t.Run("finds go.sum", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/go/go.sum", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/go", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "go.sum",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/go/pkg/mod", homedir)),
						Keys: []string{
							fmt.Sprintf("some-branch-go-%s", checksum),
							fmt.Sprintf("master-go-%s", checksum),
							fmt.Sprintf("main-go-%s", checksum),
							"some-branch-go",
							"master-go",
							"main-go",
						},
					},
				},
			},
		})
	})

	t.Run("finds yarn.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{
						Path: "node_modules",
						Keys: []string{
							fmt.Sprintf("some-branch-node-modules-%s", checksum),
							fmt.Sprintf("master-node-modules-%s", checksum),
							fmt.Sprintf("main-node-modules-%s", checksum),
							"some-branch-node-modules",
							"master-node-modules",
							"main-node-modules",
						},
					},
				},
			},
		})
	})

	t.Run("finds mix.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/elixir/mix.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/elixir", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "mix.lock",
				Entries: []LookupResultEntry{
					{Path: "deps", Keys: []string{
						fmt.Sprintf("some-branch-mix-deps-%s", checksum),
						fmt.Sprintf("master-mix-deps-%s", checksum),
						fmt.Sprintf("main-mix-deps-%s", checksum),
						"some-branch-mix-deps",
						"master-mix-deps",
						"main-mix-deps",
					}},
					{Path: "_build", Keys: []string{
						fmt.Sprintf("some-branch-mix-build-%s", checksum),
						fmt.Sprintf("master-mix-build-%s", checksum),
						fmt.Sprintf("main-mix-build-%s", checksum),
						"some-branch-mix-build",
						"master-mix-build",
						"main-mix-build",
					}},
				},
			},
		})
	})

	t.Run("finds pom.xml", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/maven/pom.xml", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/maven", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "pom.xml",
				Entries: []LookupResultEntry{
					{Path: ".m2", Keys: []string{
						fmt.Sprintf("some-branch-maven-%s", checksum),
						fmt.Sprintf("master-maven-%s", checksum),
						fmt.Sprintf("main-maven-%s", checksum),
						"some-branch-maven",
						"master-maven",
						"main-maven",
					}},
					{Path: "target", Keys: []string{
						fmt.Sprintf("some-branch-maven-target-%s", checksum),
						fmt.Sprintf("master-maven-target-%s", checksum),
						fmt.Sprintf("main-maven-target-%s", checksum),
						"some-branch-maven-target",
						"master-maven-target",
						"main-maven-target",
					}},
				},
			},
		})
	})

	t.Run("finds requirements.txt and package-lock.json", func(t *testing.T) {
		requirementsChecksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/multiple-files/requirements.txt", rootPath))
		assert.Nil(t, err)

		packageLockChecksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/multiple-files/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/multiple-files", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("some-branch-node-modules-%s", packageLockChecksum),
						fmt.Sprintf("master-node-modules-%s", packageLockChecksum),
						fmt.Sprintf("main-node-modules-%s", packageLockChecksum),
						"some-branch-node-modules",
						"master-node-modules",
						"main-node-modules",
					}},
				},
			},
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{
						fmt.Sprintf("some-branch-requirements-%s", requirementsChecksum),
						fmt.Sprintf("master-requirements-%s", requirementsChecksum),
						fmt.Sprintf("main-requirements-%s", requirementsChecksum),
						"some-branch-requirements",
						"master-requirements",
						"main-requirements",
					}},
				},
			},
		})
	})

	t.Run("no duplicated master suffixed key", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "master", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("master-node-modules-%s", checksum),
						fmt.Sprintf("main-node-modules-%s", checksum),
						"master-node-modules",
						"main-node-modules",
					}},
				},
			},
		})
	})

	t.Run("no duplicated main suffixed key", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "main", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("main-node-modules-%s", checksum),
						fmt.Sprintf("master-node-modules-%s", checksum),
						"main-node-modules",
						"master-node-modules",
					}},
				},
			},
		})
	})
}

func assertLookupResults(t *testing.T, actualResults []LookupResult, expectedResults []LookupResult) {
	if assert.Len(t, actualResults, len(expectedResults)) {
		for resultIndex, result := range actualResults {
			expectedResult := expectedResults[resultIndex]
			assert.Equal(t, result.DetectedFile, expectedResult.DetectedFile)
			if assert.Len(t, result.Entries, len(expectedResult.Entries)) {
				for index, entry := range result.Entries {
					expectedEntry := expectedResult.Entries[index]
					assert.Equal(t, entry.Path, expectedEntry.Path)
					assert.Equal(t, expectedEntry.Keys, entry.Keys)
					if expectedEntry.EquivalentKeys != nil {
						assert.Equal(t, expectedEntry.EquivalentKeys, entry.EquivalentKeys)
					}
				}
			}
		}
	}
}

func Test__LookupEquivalentKeys(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	rootPath := filepath.Dir(filepath.Dir(filepath.Dir(b)))
	lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
	checksum, err := GenerateChecksum(fmt.Sprintf("%s/package-lock.json", lookupDirectory))
	assert.Nil(t, err)

	equivalentKeysFor := func(branch string) []string {
		results := Lookup(LookupOptions{Restore: false, GitBranch: branch, LookupDirectory: lookupDirectory})
		if assert.Len(t, results, 1) && assert.Len(t, results[0].Entries, 1) {
			return results[0].Entries[0].EquivalentKeys
		}

		return nil
	}

	t.Run("feature branch checks master and main", func(t *testing.T) {
		assert.Equal(t, []string{
			fmt.Sprintf("master-node-modules-%s", checksum),
			fmt.Sprintf("main-node-modules-%s", checksum),
		}, equivalentKeysFor("some-branch"))
	})

	t.Run("master does not reference itself", func(t *testing.T) {
		assert.Equal(t, []string{fmt.Sprintf("main-node-modules-%s", checksum)}, equivalentKeysFor("master"))
	})

	t.Run("main does not reference itself", func(t *testing.T) {
		assert.Equal(t, []string{fmt.Sprintf("master-node-modules-%s", checksum)}, equivalentKeysFor("main"))
	})
}

func Test__GemsKeyRubyVersion(t *testing.T) {
	write := func(t *testing.T, dir, name, content string) {
		assert.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}

	t.Run("reads .ruby-version", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".ruby-version", "3.3.1\n")
		assert.Equal(t, "gems-ruby3.3.1-"+runtime.GOARCH, gemsKeyPrefix(dir))
	})

	t.Run("strips ruby- prefix from .ruby-version", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".ruby-version", "ruby-4.0.5")
		assert.Equal(t, "gems-ruby4.0.5-"+runtime.GOARCH, gemsKeyPrefix(dir))
	})

	t.Run("falls back to the Gemfile.lock RUBY VERSION section", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "Gemfile.lock", "GEM\n  specs:\n\nRUBY VERSION\n   ruby 3.2.2p53\n\nBUNDLED WITH\n   2.4.10\n")
		assert.Equal(t, "gems-ruby3.2.2-"+runtime.GOARCH, gemsKeyPrefix(dir))
	})

	t.Run(".ruby-version wins over Gemfile.lock", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, ".ruby-version", "3.3.1")
		write(t, dir, "Gemfile.lock", "RUBY VERSION\n   ruby 3.2.2p53\n")
		assert.Equal(t, "gems-ruby3.3.1-"+runtime.GOARCH, gemsKeyPrefix(dir))
	})

	t.Run("uses RBENV_VERSION last", func(t *testing.T) {
		t.Setenv("RBENV_VERSION", "3.1.4")
		assert.Equal(t, "gems-ruby3.1.4-"+runtime.GOARCH, gemsKeyPrefix(t.TempDir()))
	})

	t.Run("omits the version when unknown", func(t *testing.T) {
		t.Setenv("RBENV_VERSION", "")
		assert.Equal(t, "gems-"+runtime.GOARCH, gemsKeyPrefix(t.TempDir()))
	})

	t.Run("ignores unsafe version strings", func(t *testing.T) {
		t.Setenv("RBENV_VERSION", "")
		dir := t.TempDir()
		write(t, dir, ".ruby-version", "3.3.1/../../etc")
		assert.Equal(t, "gems-"+runtime.GOARCH, gemsKeyPrefix(dir))
	})
}
