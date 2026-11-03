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
	GitBranch       string
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

	results := []LookupResult{}
	for _, lockFile := range lockFiles {
		lockFilePath := fmt.Sprintf("%s/%s", lookupDirectory, lockFile)
		if _, err := os.Stat(lockFilePath); err == nil {
			resultForFile := resultForfile(lockFilePath, options)
			results = append(results, *resultForFile)
		}
	}

	return results
}

func resultForfile(filePath string, options LookupOptions) *LookupResult {
	homedir := options.HomeDirectory
	if homedir == "" {
		homedir, _ = os.UserHomeDir()
	}

	file := filepath.Base(filePath)

	switch file {
	case ".nvmrc":
		return buildResult(filePath, options, []buildResultRequest{
			{"nvm", fmt.Sprintf("%s/.nvm", homedir)},
		})
	case "Gemfile.lock":
		return buildResult(filePath, options, []buildResultRequest{
			{gemsKeyPrefix(filepath.Dir(filePath)), "vendor/bundle"},
		})
	case "package-lock.json":
		return buildResult(filePath, options, []buildResultRequest{
			{"node-modules", "node_modules"},
		})
	case "yarn.lock":
		// Only node_modules is cached: it makes `yarn install` instant on a
		// cache hit, so restoring ~/.cache/yarn (~1.4 GB) adds no benefit.
		return buildResult(filePath, options, []buildResultRequest{
			{"node-modules", "node_modules"},
		})
	case "mix.lock":
		return buildResult(filePath, options, []buildResultRequest{
			{"mix-deps", "deps"},
			{"mix-build", "_build"},
		})
	case "requirements.txt":
		return buildResult(filePath, options, []buildResultRequest{
			{"requirements", ".pip_cache"},
		})
	case "composer.lock":
		return buildResult(filePath, options, []buildResultRequest{
			{"composer", "vendor"},
		})
	case "pom.xml":
		return buildResult(filePath, options, []buildResultRequest{
			{"maven", ".m2"},
			{"maven-target", "target"},
		})
	case "Podfile.lock":
		return buildResult(filePath, options, []buildResultRequest{
			{"pods", "Pods"},
		})
	case "go.sum":
		return buildResult(filePath, options, []buildResultRequest{
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

func buildResult(filePath string, options LookupOptions, entries []buildResultRequest) *LookupResult {
	gitBranch := options.GitBranch
	if gitBranch == "" {
		gitBranch = "master"
	}

	checksum, err := GenerateChecksum(filePath)
	if err != nil {
		log.Errorf("Error generating checksum for %s: %v", filePath, err)
		return nil
	}

	newEntries := []LookupResultEntry{}
	for _, entry := range entries {
		if options.Restore {
			newEntries = append(newEntries, LookupResultEntry{
				Path: filepath.FromSlash(entry.Path),
				Keys: keysForRestore(entry.KeyPrefix, gitBranch, checksum),
			})
		} else {
			key := fmt.Sprintf("%s-%s-%s", gitBranch, entry.KeyPrefix, checksum)
			newEntries = append(newEntries, LookupResultEntry{
				Keys:           []string{key},
				Path:           filepath.FromSlash(entry.Path),
				EquivalentKeys: equivalentKeys(entry.KeyPrefix, gitBranch, checksum),
			})
		}
	}

	return &LookupResult{
		DetectedFile: filepath.Base(filePath),
		Entries:      newEntries,
	}
}

func keysForRestore(keyPrefix, gitBranch, checksum string) []string {
	keys := []string{fmt.Sprintf("%s-%s-%s", gitBranch, keyPrefix, checksum)}

	// Exact default-branch keys: a single HEAD hit, no listing needed.
	keys = append(keys, equivalentKeys(keyPrefix, gitBranch, checksum)...)
	keys = append(keys, fmt.Sprintf("%s-%s", gitBranch, keyPrefix))

	if gitBranch != "master" {
		keys = append(keys, fmt.Sprintf("master-%s", keyPrefix))
	}

	if gitBranch != "main" {
		keys = append(keys, fmt.Sprintf("main-%s", keyPrefix))
	}

	return keys
}

func equivalentKeys(keyPrefix, gitBranch, checksum string) []string {
	keys := []string{}
	for _, defaultBranch := range []string{"master", "main"} {
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
