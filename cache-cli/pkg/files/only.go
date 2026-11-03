package files

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry types per lock file, in the order resultForfile builds its entries.
var lockFileTypes = map[string][]string{
	".nvmrc":            {"nvm"},
	"Gemfile.lock":      {"gems"},
	"package-lock.json": {"node-modules"},
	"yarn.lock":         {"node-modules"},
	"mix.lock":          {"mix-deps", "mix-build"},
	"requirements.txt":  {"requirements"},
	"composer.lock":     {"composer"},
	"pom.xml":           {"maven", "maven-target"},
	"Podfile.lock":      {"pods"},
	"go.sum":            {"go"},
}

// Types only built on request; automatic mode without --only never stores them.
var onDemandTypes = map[string]buildResultRequest{
	"bootsnap": {"bootsnap", "tmp/bootsnap-cache"},
}

var onDemandLockFiles = map[string]string{
	"bootsnap": "Gemfile.lock",
}

func EntryTypes() []string {
	seen := map[string]bool{}
	for _, types := range lockFileTypes {
		for _, entryType := range types {
			seen[entryType] = true
		}
	}

	for entryType := range onDemandTypes {
		seen[entryType] = true
	}

	types := make([]string, 0, len(seen))
	for entryType := range seen {
		types = append(types, entryType)
	}

	sort.Strings(types)
	return types
}

// ParseEntryTypes parses a comma-separated --only value.
func ParseEntryTypes(value string) ([]string, error) {
	known := map[string]bool{}
	for _, entryType := range EntryTypes() {
		known[entryType] = true
	}

	types := []string{}
	seen := map[string]bool{}
	for _, raw := range strings.Split(value, ",") {
		entryType := strings.TrimSpace(raw)
		if entryType == "" || seen[entryType] {
			continue
		}

		if !known[entryType] {
			return nil, fmt.Errorf("unknown cache type '%s'; valid types: %s", entryType, strings.Join(EntryTypes(), ", "))
		}

		seen[entryType] = true
		types = append(types, entryType)
	}

	if len(types) == 0 {
		return nil, fmt.Errorf("--only needs at least one type; valid types: %s", strings.Join(EntryTypes(), ", "))
	}

	return types, nil
}

// LookupTypes is Lookup restricted to the given entry types.
// It returns the results and the types that had no lock file.
func LookupTypes(options LookupOptions, types []string) ([]LookupResult, []string) {
	lookupDirectory := options.LookupDirectory
	if lookupDirectory == "" {
		lookupDirectory, _ = os.Getwd()
	}

	scope := directoryScope(options.RootDirectory, lookupDirectory)
	selected := map[string]bool{}
	for _, entryType := range types {
		selected[entryType] = true
	}

	found := map[string]bool{}
	results := []LookupResult{}
	for _, lockFile := range lockFiles {
		if !anySelected(lockFileTypes[lockFile], selected) {
			continue
		}

		lockFilePath := filepath.Join(lookupDirectory, lockFile)
		if _, err := os.Stat(lockFilePath); err != nil {
			continue
		}

		if result := resultForfile(lockFilePath, scope, options); result != nil {
			if filtered := filterEntries(*result, selected, found); len(filtered.Entries) > 0 {
				results = append(results, filtered)
			}
		}
	}

	for _, entryType := range types {
		request, ok := onDemandTypes[entryType]
		if !ok {
			continue
		}

		lockFilePath := filepath.Join(lookupDirectory, onDemandLockFiles[entryType])
		if _, err := os.Stat(lockFilePath); err != nil {
			continue
		}

		if result := buildResult(lockFilePath, scope, options, []buildResultRequest{request}); result != nil {
			found[entryType] = true
			results = appendResult(results, *result)
		}
	}

	missing := []string{}
	for _, entryType := range types {
		if !found[entryType] {
			missing = append(missing, entryType)
		}
	}

	return dropShadowedPaths(results), missing
}

// Entries for an already detected lock file join its result.
func appendResult(results []LookupResult, result LookupResult) []LookupResult {
	for index := range results {
		if results[index].DetectedFile == result.DetectedFile {
			results[index].Entries = append(results[index].Entries, result.Entries...)
			return results
		}
	}

	return append(results, result)
}

func anySelected(types []string, selected map[string]bool) bool {
	for _, entryType := range types {
		if selected[entryType] {
			return true
		}
	}

	return false
}

func filterEntries(result LookupResult, selected, found map[string]bool) LookupResult {
	types := lockFileTypes[result.DetectedFile]
	filtered := LookupResult{DetectedFile: result.DetectedFile}
	for index, entry := range result.Entries {
		if index < len(types) && selected[types[index]] {
			found[types[index]] = true
			filtered.Entries = append(filtered.Entries, entry)
		}
	}

	return filtered
}

// yarn.lock and package-lock.json both map to node_modules; the later lock file wins.
func dropShadowedPaths(results []LookupResult) []LookupResult {
	lastOwner := map[string]int{}
	for index, result := range results {
		for _, entry := range result.Entries {
			lastOwner[entry.Path] = index
		}
	}

	kept := []LookupResult{}
	for index, result := range results {
		entries := []LookupResultEntry{}
		for _, entry := range result.Entries {
			if lastOwner[entry.Path] == index {
				entries = append(entries, entry)
			}
		}

		if len(entries) > 0 {
			result.Entries = entries
			kept = append(kept, result)
		}
	}

	return kept
}
