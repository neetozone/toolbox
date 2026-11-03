package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/logging"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/storage"
	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

// Runs against the local backend so it needs no sftp/minio containers.
func Test__SkipStoreAndExactRestore(t *testing.T) {
	log.SetFormatter(new(logging.CustomFormatter))
	log.SetLevel(log.InfoLevel)
	log.SetOutput(openLogfileForTests(t))

	cacheDir := t.TempDir()
	t.Setenv("NEETO_CI_CACHE_BACKEND", "local")
	t.Setenv("NEETO_CI_LOCAL_CACHE_DIR", cacheDir)
	t.Setenv("NEETO_CI_CACHE_ARCHIVE_METHOD", "native")
	t.Setenv("NEETO_CI_GIT_PR_BRANCH", "")
	t.Setenv("RBENV_VERSION", "")

	store, err := storage.InitStorage()
	assert.NoError(t, err)

	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	newProject := func(t *testing.T, lockfile string) string {
		dir := t.TempDir()
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "Gemfile.lock"), []byte(lockfile), 0600))
		assert.NoError(t, os.MkdirAll(filepath.Join(dir, "vendor", "bundle"), 0755))
		assert.NoError(t, os.WriteFile(filepath.Join(dir, "vendor", "bundle", "gem.rb"), []byte("puts 1"), 0600))
		assert.NoError(t, os.Chdir(dir))
		return dir
	}

	storeOn := func(branch string) string {
		t.Setenv("NEETO_CI_GIT_BRANCH", branch)
		RunStore(NewStoreCommand(), []string{})
		return readOutputFromFile(t)
	}

	keyNames := func() []string {
		keys, err := store.List()
		assert.NoError(t, err)
		names := []string{}
		for _, key := range keys {
			names = append(names, key.Name)
		}

		return names
	}

	t.Run("feature branch skips the store when master has identical content", func(t *testing.T) {
		assert.NoError(t, store.Clear())
		newProject(t, "lock-v1")
		checksum, _ := files.GenerateChecksum("Gemfile.lock")
		masterKey := fmt.Sprintf("master-gems-%s-%s", runtime.GOARCH, checksum)

		assert.Contains(t, storeOn("master"), "Upload complete")
		assert.Equal(t, []string{masterKey}, keyNames())

		output := storeOn("some-feature")
		assert.Contains(t, output, "identical content already cached")
		assert.NotContains(t, output, "Compressing")
		assert.Equal(t, []string{masterKey}, keyNames())
	})

	t.Run("feature branch stores when the lockfile differs from master", func(t *testing.T) {
		assert.NoError(t, store.Clear())
		newProject(t, "lock-v1")
		storeOn("master")

		newProject(t, "lock-v2")
		output := storeOn("some-feature")
		assert.Contains(t, output, "Upload complete")
		assert.NotContains(t, output, "identical content")
		assert.Len(t, keyNames(), 2)
	})

	t.Run("the store is not skipped when the branch key already exists on main", func(t *testing.T) {
		assert.NoError(t, store.Clear())
		newProject(t, "lock-v1")
		storeOn("main")

		assert.Contains(t, storeOn("some-feature"), "identical content already cached")
	})

	t.Run("restore hits the exact default-branch key without a miss", func(t *testing.T) {
		assert.NoError(t, store.Clear())
		dir := newProject(t, "lock-v1")
		checksum, _ := files.GenerateChecksum("Gemfile.lock")
		masterKey := fmt.Sprintf("master-gems-%s-%s", runtime.GOARCH, checksum)
		storeOn("master")

		assert.NoError(t, os.RemoveAll(filepath.Join(dir, "vendor")))
		t.Setenv("NEETO_CI_GIT_BRANCH", "some-feature")
		RunRestore(restoreCmd, []string{})
		output := readOutputFromFile(t)

		assert.Contains(t, output, fmt.Sprintf("HIT: '%s', using key '%s'", masterKey, masterKey))
		assert.Contains(t, output, fmt.Sprintf("MISS: 'some-feature-gems-%s-%s'", runtime.GOARCH, checksum))
		assert.NotContains(t, output, fmt.Sprintf("MISS: 'some-feature-gems-%s'.", runtime.GOARCH))
		restored, err := os.ReadFile(filepath.Join(dir, "vendor", "bundle", "gem.rb"))
		assert.NoError(t, err)
		assert.Equal(t, "puts 1", string(restored))
	})

	t.Run("restore falls back to the newest default-branch key for a changed lockfile", func(t *testing.T) {
		assert.NoError(t, store.Clear())
		newProject(t, "lock-v1")
		checksum, _ := files.GenerateChecksum("Gemfile.lock")
		storeOn("master")

		dir := newProject(t, "lock-v2")
		t.Setenv("NEETO_CI_GIT_BRANCH", "some-feature")
		RunRestore(restoreCmd, []string{})
		output := readOutputFromFile(t)

		assert.Contains(t, output, fmt.Sprintf("using key 'master-gems-%s-%s'", runtime.GOARCH, checksum))
		_, err := os.Stat(filepath.Join(dir, "vendor", "bundle", "gem.rb"))
		assert.NoError(t, err)
	})
}

func Test__FindMatchingKeyIsLiteralPrefix(t *testing.T) {
	keys := []storage.CacheKey{
		{Name: "feature-master-gems-arm64-abc"},
		{Name: "master-gems-arm64-abc"},
		{Name: "master-gems-ruby3x3x1-arm64-abc"},
	}

	assert.Equal(t, "master-gems-arm64-abc", findMatchingKey(keys, "master-gems"))
	assert.Equal(t, "", findMatchingKey(keys, "main-gems"))
	assert.Equal(t, "", findMatchingKey(keys, "master-gems-ruby3.3.1"), "dots are literal, not regex wildcards")
	assert.Equal(t, "master-gems-ruby3x3x1-arm64-abc", findMatchingKey(keys, "master-gems-ruby3x3"))
}
