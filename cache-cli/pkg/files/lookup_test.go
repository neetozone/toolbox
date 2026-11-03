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
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: ".nvmrc",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/.nvm", homedir)),
						Keys: []string{fmt.Sprintf("some-branch-nvm-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds Gemfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/gems/Gemfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/gems", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Gemfile.lock",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash("vendor/bundle"),
						Keys: []string{fmt.Sprintf("some-branch-gems-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds package-lock.json", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("some-branch-node-modules-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds requirements.txt", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/pip/requirements.txt", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/pip", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{fmt.Sprintf("some-branch-requirements-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds composer.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/composer/composer.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/composer", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "composer.lock",
				Entries: []LookupResultEntry{
					{Path: "vendor", Keys: []string{fmt.Sprintf("some-branch-composer-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds Podfile.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/cocoapods/Podfile.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/cocoapods", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "Podfile.lock",
				Entries: []LookupResultEntry{
					{Path: "Pods", Keys: []string{fmt.Sprintf("some-branch-pods-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds go.sum", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/go/go.sum", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/go", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "go.sum",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/go/pkg/mod", homedir)),
						Keys: []string{fmt.Sprintf("some-branch-go-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds yarn.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{
						Path: "node_modules",
						Keys: []string{fmt.Sprintf("some-branch-node-modules-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("finds mix.lock", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/elixir/mix.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/elixir", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "mix.lock",
				Entries: []LookupResultEntry{
					{Path: "deps", Keys: []string{fmt.Sprintf("some-branch-mix-deps-%s", checksum)}},
					{Path: "_build", Keys: []string{fmt.Sprintf("some-branch-mix-build-%s", checksum)}},
				},
			},
		})
	})

	t.Run("finds pom.xml", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/maven/pom.xml", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/maven", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "pom.xml",
				Entries: []LookupResultEntry{
					{Path: ".m2", Keys: []string{fmt.Sprintf("some-branch-maven-%s", checksum)}},
					{Path: "target", Keys: []string{fmt.Sprintf("some-branch-maven-target-%s", checksum)}},
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
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("some-branch-node-modules-%s", packageLockChecksum)}},
				},
			},
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{fmt.Sprintf("some-branch-requirements-%s", requirementsChecksum)}},
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
							"some-branch-nvm",
							"main-nvm",
							"master-nvm",
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
							fmt.Sprintf("some-branch-gems-%s", checksum),
							"some-branch-gems",
							"main-gems",
							"master-gems",
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
						"some-branch-node-modules",
						"main-node-modules",
						"master-node-modules",
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
						"some-branch-requirements",
						"main-requirements",
						"master-requirements",
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
						"some-branch-composer",
						"main-composer",
						"master-composer",
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
						"some-branch-pods",
						"main-pods",
						"master-pods",
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
							"some-branch-go",
							"main-go",
							"master-go",
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
							"some-branch-node-modules",
							"main-node-modules",
							"master-node-modules",
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
						"some-branch-mix-deps",
						"main-mix-deps",
						"master-mix-deps",
					}},
					{Path: "_build", Keys: []string{
						fmt.Sprintf("some-branch-mix-build-%s", checksum),
						"some-branch-mix-build",
						"main-mix-build",
						"master-mix-build",
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
						"some-branch-maven",
						"main-maven",
						"master-maven",
					}},
					{Path: "target", Keys: []string{
						fmt.Sprintf("some-branch-maven-target-%s", checksum),
						"some-branch-maven-target",
						"main-maven-target",
						"master-maven-target",
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
						"some-branch-node-modules",
						"main-node-modules",
						"master-node-modules",
					}},
				},
			},
			{
				DetectedFile: "requirements.txt",
				Entries: []LookupResultEntry{
					{Path: ".pip_cache", Keys: []string{
						fmt.Sprintf("some-branch-requirements-%s", requirementsChecksum),
						"some-branch-requirements",
						"main-requirements",
						"master-requirements",
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
						"main-node-modules",
						"master-node-modules",
					}},
				},
			},
		})
	})

	t.Run("only default branch fallbacks without a branch", func(t *testing.T) {
		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						"main-node-modules",
						"master-node-modules",
					}},
				},
			},
		})
	})

	t.Run("falls back to the configured default branch only", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/npm/package-lock.json", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", DefaultBranch: "develop", LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "package-lock.json",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("some-branch-node-modules-%s", checksum),
						"some-branch-node-modules",
						"develop-node-modules",
					}},
				},
			},
		})
	})
}

func Test__LookupDirectoryScope(t *testing.T) {
	homedir, _ := os.UserHomeDir()

	_, b, _, _ := runtime.Caller(0)
	testFilePath := filepath.Dir(b)
	pkgPath := filepath.Dir(testFilePath)
	rootPath := filepath.Dir(pkgPath)

	t.Run("keeps keys unscoped in the root directory", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", RootDirectory: lookupDirectory, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("some-branch-node-modules-%s", checksum)}},
				},
			},
		})
	})

	t.Run("scopes store keys of a nested directory", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", RootDirectory: rootPath, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("some-branch-node-modules@test-autocache-yarn-%s", checksum)}},
				},
			},
		})
	})

	t.Run("scopes every restore key of a nested directory", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		rootDirectory := fmt.Sprintf("%s/test/autocache", rootPath)
		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: true, GitBranch: "some-branch", RootDirectory: rootDirectory, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{
						fmt.Sprintf("some-branch-node-modules@yarn-%s", checksum),
						"some-branch-node-modules@yarn",
						"main-node-modules@yarn",
						"master-node-modules@yarn",
					}},
				},
			},
		})
	})

	t.Run("does not scope paths outside the directory", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/nvm/.nvmrc", rootPath))
		assert.Nil(t, err)

		lookupDirectory := fmt.Sprintf("%s/test/autocache/nvm", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", RootDirectory: rootPath, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: ".nvmrc",
				Entries: []LookupResultEntry{
					{
						Path: filepath.FromSlash(fmt.Sprintf("%s/.nvm", homedir)),
						Keys: []string{fmt.Sprintf("some-branch-nvm-%s", checksum)},
					},
				},
			},
		})
	})

	t.Run("ignores a root directory outside the lookup directory", func(t *testing.T) {
		checksum, err := GenerateChecksum(fmt.Sprintf("%s/test/autocache/yarn/yarn.lock", rootPath))
		assert.Nil(t, err)

		rootDirectory := fmt.Sprintf("%s/test/autocache/npm", rootPath)
		lookupDirectory := fmt.Sprintf("%s/test/autocache/yarn", rootPath)
		assertLookupResults(t, Lookup(LookupOptions{Restore: false, GitBranch: "some-branch", RootDirectory: rootDirectory, LookupDirectory: lookupDirectory}), []LookupResult{
			{
				DetectedFile: "yarn.lock",
				Entries: []LookupResultEntry{
					{Path: "node_modules", Keys: []string{fmt.Sprintf("some-branch-node-modules-%s", checksum)}},
				},
			},
		})
	})
}

func Test__IsAutomaticKeyFor(t *testing.T) {
	checksum := "a0f1a9a1d9776398da24a9857f85ccc7"

	assert.True(t, IsAutomaticKeyFor("main-node-modules", "main-node-modules-"+checksum))
	assert.True(t, IsAutomaticKeyFor("main-node-modules@e2e", "main-node-modules@e2e-"+checksum))

	assert.False(t, IsAutomaticKeyFor("main-node-modules", "main-node-modules@e2e-"+checksum))
	assert.False(t, IsAutomaticKeyFor("main-node-modules@e2e", "main-node-modules-"+checksum))
	assert.False(t, IsAutomaticKeyFor("main-node-modules", "release-main-node-modules-"+checksum))
	assert.False(t, IsAutomaticKeyFor("main-node-modules", "main-node-modules-"+checksum+"-old"))
	assert.False(t, IsAutomaticKeyFor("main-node-modules", "main-node-modules"))
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
					assert.Equal(t, entry.Keys, expectedEntry.Keys)
				}
			}
		}
	}
}
