package files

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	log "github.com/sirupsen/logrus"
)

var (
	checksumPattern       = regexp.MustCompile(`^[0-9a-f]{32}$`)
	unsafeScopeCharacters = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
)

var lockFiles = []string{
	".nvmrc",
	"Gemfile.lock",
	"package-lock.json",
	"yarn.lock",
	"mix.lock",
	"requirements.txt",
	"composer.lock",
	"pom.xml",
	"Podfile.lock",
	"go.sum",
}

type LookupOptions struct {
	LookupDirectory string
	HomeDirectory   string
	RootDirectory   string
	GitBranch       string
	DefaultBranch   string
	Restore         bool
}

type LookupResult struct {
	DetectedFile string
	Entries      []LookupResultEntry
}

type LookupResultEntry struct {
	Keys []string
	Path string
	// Default-branch keys with the same checksum; storing is skipped if one exists.
	EquivalentKeys []string
}

func Lookup(options LookupOptions) []LookupResult {
	lookupDirectory := options.LookupDirectory
	if lookupDirectory == "" {
		lookupDirectory, _ = os.Getwd()
	}

	scope := directoryScope(options.RootDirectory, lookupDirectory)

	results := []LookupResult{}
	for _, lockFile := range lockFiles {
		lockFilePath := fmt.Sprintf("%s/%s", lookupDirectory, lockFile)
		if _, err := os.Stat(lockFilePath); err == nil {
			resultForFile := resultForfile(lockFilePath, scope, options)
			if resultForFile == nil {
				log.Warnf("Skipping %s: no cache keys could be generated for it.", lockFilePath)
				continue
			}

			results = append(results, *resultForFile)
		}
	}

	return results
}

// Keys from a nested directory carry its repository-relative path, so a
// fallback for one directory never restores another directory's archive.
// The repository root keeps unscoped keys.
func directoryScope(rootDirectory, lookupDirectory string) string {
	if rootDirectory == "" {
		return ""
	}

	root, err := filepath.EvalSymlinks(rootDirectory)
	if err != nil {
		return ""
	}

	directory, err := filepath.EvalSymlinks(lookupDirectory)
	if err != nil {
		return ""
	}

	relative, err := filepath.Rel(root, directory)
	if err != nil || relative == "." || strings.HasPrefix(relative, "..") {
		return ""
	}

	return unsafeScopeCharacters.ReplaceAllString(filepath.ToSlash(relative), "-")
}

func resultForfile(filePath, scope string, options LookupOptions) *LookupResult {
	homedir := options.HomeDirectory
	if homedir == "" {
		homedir, _ = os.UserHomeDir()
	}

	file := filepath.Base(filePath)

	switch file {
	case ".nvmrc":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"nvm", fmt.Sprintf("%s/.nvm", homedir)},
		})
	case "Gemfile.lock":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{gemsKeyPrefix(filepath.Dir(filePath)), "vendor/bundle"},
		})
	case "package-lock.json":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"node-modules", "node_modules"},
		})
	case "yarn.lock":
		// Only node_modules is cached: it makes `yarn install` instant on a
		// cache hit, so restoring ~/.cache/yarn (~1.4 GB) adds no benefit.
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"node-modules", "node_modules"},
		})
	case "mix.lock":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"mix-deps", "deps"},
			{"mix-build", "_build"},
		})
	case "requirements.txt":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"requirements", ".pip_cache"},
		})
	case "composer.lock":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"composer", "vendor"},
		})
	case "pom.xml":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"maven", ".m2"},
			{"maven-target", "target"},
		})
	case "Podfile.lock":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"pods", "Pods"},
		})
	case "go.sum":
		return buildResult(filePath, scope, options, []buildResultRequest{
			{"go", fmt.Sprintf("%s/go/pkg/mod", homedir)},
		})
	default:
		fmt.Printf("Missing branch for %s - this should never happen!\n", file)
		return nil
	}
}

type buildResultRequest struct {
	KeyPrefix string
	Path      string
}

func buildResult(filePath, scope string, options LookupOptions, entries []buildResultRequest) *LookupResult {
	gitBranch := options.GitBranch

	checksum, err := GenerateChecksum(filePath)
	if err != nil {
		log.Warnf("Error generating checksum for %s: %v", filePath, err)
		return nil
	}

	newEntries := []LookupResultEntry{}
	for _, entry := range entries {
		keyPrefix := scopedKeyPrefix(entry, scope)

		if options.Restore {
			newEntries = append(newEntries, LookupResultEntry{
				Path: filepath.FromSlash(entry.Path),
				Keys: keysForRestore(keyPrefix, gitBranch, checksum, defaultBranches(options)),
			})
		} else {
			key := fmt.Sprintf("%s-%s-%s", gitBranch, keyPrefix, checksum)
			newEntries = append(newEntries, LookupResultEntry{
				Keys:           []string{key},
				Path:           filepath.FromSlash(entry.Path),
				EquivalentKeys: equivalentKeys(keyPrefix, gitBranch, checksum, defaultBranches(options)),
			})
		}
	}

	return &LookupResult{
		DetectedFile: filepath.Base(filePath),
		Entries:      newEntries,
	}
}

// Paths outside the directory, like ~/.nvm, hold the same content whichever
// directory restores them, so only relative paths are scoped.
func scopedKeyPrefix(entry buildResultRequest, scope string) string {
	if scope == "" || filepath.IsAbs(entry.Path) {
		return entry.KeyPrefix
	}

	return fmt.Sprintf("%s@%s", entry.KeyPrefix, scope)
}

func defaultBranches(options LookupOptions) []string {
	if options.DefaultBranch != "" {
		return []string{options.DefaultBranch}
	}

	return []string{"main", "master"}
}

func keysForRestore(keyPrefix, gitBranch, checksum string, defaultBranches []string) []string {
	keys := []string{}
	if gitBranch != "" {
		keys = append(keys, fmt.Sprintf("%s-%s-%s", gitBranch, keyPrefix, checksum))
	}

	// Exact default-branch keys: a single HEAD hit, no listing needed.
	keys = append(keys, equivalentKeys(keyPrefix, gitBranch, checksum, defaultBranches)...)
	if gitBranch != "" {
		keys = append(keys, fmt.Sprintf("%s-%s", gitBranch, keyPrefix))
	}

	for _, defaultBranch := range defaultBranches {
		if defaultBranch != gitBranch {
			keys = append(keys, fmt.Sprintf("%s-%s", defaultBranch, keyPrefix))
		}
	}

	return keys
}

// Automatic fallback keys only match '<fallback>-<checksum>', so
// 'main-node-modules' matches neither 'main-node-modules@e2e-<checksum>' nor
// 'release-main-node-modules-<checksum>'.
func IsAutomaticKeyFor(fallbackKey, key string) bool {
	prefix := fallbackKey + "-"
	return strings.HasPrefix(key, prefix) && checksumPattern.MatchString(strings.TrimPrefix(key, prefix))
}

func equivalentKeys(keyPrefix, gitBranch, checksum string, defaultBranches []string) []string {
	keys := []string{}
	for _, defaultBranch := range defaultBranches {
		if gitBranch != defaultBranch {
			keys = append(keys, fmt.Sprintf("%s-%s-%s", defaultBranch, keyPrefix, checksum))
		}
	}

	return keys
}

// gemsKeyPrefix scopes the gems key to the Ruby version and arch, so a
// fallback never restores gems built for another Ruby or architecture.
func gemsKeyPrefix(dir string) string {
	prefix := "gems"
	if version := rubyVersion(dir); version != "" {
		prefix += "-ruby" + version
	}

	return prefix + "-" + runtime.GOARCH
}

var (
	rubyVersionFormat   = regexp.MustCompile(`^[0-9A-Za-z._]+$`)
	gemfileLockRubyLine = regexp.MustCompile(`(?m)^RUBY VERSION\s*\n\s+ruby (\d+(?:\.\d+)*)`)
)

func rubyVersion(dir string) string {
	if content, err := os.ReadFile(filepath.Join(dir, ".ruby-version")); err == nil {
		version := strings.TrimPrefix(strings.TrimSpace(string(content)), "ruby-")
		if rubyVersionFormat.MatchString(version) {
			return version
		}
	}

	if content, err := os.ReadFile(filepath.Join(dir, "Gemfile.lock")); err == nil {
		if match := gemfileLockRubyLine.FindSubmatch(content); match != nil {
			return string(match[1])
		}
	}

	version := strings.TrimPrefix(strings.TrimSpace(os.Getenv("RBENV_VERSION")), "ruby-")
	if rubyVersionFormat.MatchString(version) {
		return version
	}

	return ""
}
