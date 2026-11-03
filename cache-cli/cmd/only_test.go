package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	assert "github.com/stretchr/testify/assert"
)

func newOnlyCommand(t *testing.T, only string) *cobra.Command {
	cmd := &cobra.Command{}
	addOnlyFlag(cmd)
	if only != "" {
		assert.Nil(t, cmd.Flags().Set(onlyFlag, only))
	}

	return cmd
}

func Test__OnlyFlagIsRegistered(t *testing.T) {
	assert.NotNil(t, restoreCmd.Flags().Lookup(onlyFlag))
	assert.NotNil(t, NewStoreCommand().Flags().Lookup(onlyFlag))
}

func Test__OnlyTypes(t *testing.T) {
	types, err := onlyTypes(newOnlyCommand(t, ""), []string{})
	assert.Nil(t, err)
	assert.Nil(t, types)

	types, err = onlyTypes(newOnlyCommand(t, ""), []string{"some-key"})
	assert.Nil(t, err)
	assert.Nil(t, types)

	types, err = onlyTypes(newOnlyCommand(t, "gems,nvm,bootsnap"), []string{})
	assert.Nil(t, err)
	assert.Equal(t, []string{"gems", "nvm", "bootsnap"}, types)

	_, err = onlyTypes(newOnlyCommand(t, "gems"), []string{"some-key"})
	assert.Contains(t, fmt.Sprint(err), "only in automatic mode")

	_, err = onlyTypes(newOnlyCommand(t, "gems"), []string{"some-key", "some/path"})
	assert.Contains(t, fmt.Sprint(err), "only in automatic mode")

	_, err = onlyTypes(newOnlyCommand(t, "gems,foo"), []string{})
	assert.Contains(t, fmt.Sprint(err), "unknown cache type 'foo'")

	cmd := &cobra.Command{}
	addOnlyFlag(cmd)
	assert.Nil(t, cmd.Flags().Parse([]string{"--only="}))
	_, err = onlyTypes(cmd, []string{})
	assert.Contains(t, fmt.Sprint(err), "at least one type")
}

// Usage errors run in a child process because they exit.
func Test__OnlyUsageErrorsExitOne(t *testing.T) {
	if mode := os.Getenv("CACHE_ONLY_EXIT_TEST"); mode != "" {
		switch mode {
		case "restore-unknown":
			assert.Nil(t, restoreCmd.Flags().Set(onlyFlag, "foo"))
			RunRestore(restoreCmd, []string{})
		case "restore-key":
			assert.Nil(t, restoreCmd.Flags().Set(onlyFlag, "gems"))
			RunRestore(restoreCmd, []string{"some-key"})
		case "store-key":
			cmd := NewStoreCommand()
			assert.Nil(t, cmd.Flags().Set(onlyFlag, "gems"))
			RunStore(cmd, []string{"some-key", "some-path"})
		}

		os.Exit(0)
	}

	for _, mode := range []string{"restore-unknown", "restore-key", "store-key"} {
		child := exec.Command(os.Args[0], "-test.run=^Test__OnlyUsageErrorsExitOne$")
		child.Env = append(os.Environ(), "CACHE_ONLY_EXIT_TEST="+mode, "NEETO_CI_CACHE_BACKEND=")
		out, err := child.CombinedOutput()
		assert.Contains(t, string(out), "level=error", mode)
		exitErr, ok := err.(*exec.ExitError)
		assert.True(t, ok, mode)
		if ok {
			assert.Equal(t, 1, exitErr.ExitCode(), mode)
		}
	}
}

func Test__OnlyStoreAndRestoreWithLocalBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses tar")
	}

	var output bytes.Buffer
	previous := log.StandardLogger().Out
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	workdir := t.TempDir()
	t.Setenv("NEETO_CI_CACHE_BACKEND", "local")
	t.Setenv("NEETO_CI_LOCAL_CACHE_DIR", t.TempDir())
	t.Setenv("NEETO_CI_GIT_BRANCH", "")
	t.Setenv("NEETO_CI_DEFAULT_BRANCH", "main")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(workdir))
	t.Setenv("NEETO_CI_GIT_PR_BRANCH", "main")

	previousDir, _ := os.Getwd()
	assert.Nil(t, os.Chdir(workdir))
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	assert.Nil(t, os.WriteFile("Gemfile.lock", []byte("GEM\n"), 0600))
	assert.Nil(t, os.WriteFile("yarn.lock", []byte("# yarn\n"), 0600))
	assert.Nil(t, os.MkdirAll("vendor/bundle", 0700))
	assert.Nil(t, os.MkdirAll("node_modules", 0700))
	assert.Nil(t, os.WriteFile("vendor/bundle/marker", []byte("GEMS"), 0600))
	assert.Nil(t, os.WriteFile("node_modules/marker", []byte("NODE"), 0600))

	storeCmd := NewStoreCommand()
	assert.Nil(t, storeCmd.Flags().Set(onlyFlag, "gems,pods"))
	RunStore(storeCmd, []string{})

	checksum, _ := files.GenerateChecksum("Gemfile.lock")
	gemsKey := fmt.Sprintf("main-gems-%s-%s", runtime.GOARCH, checksum)
	stored, _ := os.ReadDir(os.Getenv("NEETO_CI_LOCAL_CACHE_DIR"))
	names := []string{}
	for _, entry := range stored {
		names = append(names, entry.Name())
	}

	assert.Equal(t, []string{gemsKey}, names)
	assert.Contains(t, output.String(), "Skipping pods cache: no lock file for it in this directory.")

	assert.Nil(t, os.RemoveAll("vendor"))
	assert.Nil(t, os.Remove("node_modules/marker"))
	t.Setenv("NEETO_CI_GIT_PR_BRANCH", "feature")
	restore := &cobra.Command{Run: restoreCmd.Run}
	addOnlyFlag(restore)
	assert.Nil(t, restore.Flags().Set(onlyFlag, "gems"))
	RunRestore(restore, []string{})

	marker, err := os.ReadFile("vendor/bundle/marker")
	assert.Nil(t, err)
	assert.Equal(t, "GEMS", string(marker))
	assert.Contains(t, output.String(), fmt.Sprintf("HIT: 'main-gems-%s-%s'", runtime.GOARCH, checksum))
	assert.NotContains(t, output.String(), "Detected yarn.lock")
}

func Test__LookupAutomaticSaysWhenALockFileCannotBeRead(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("chmod 000 does not block root")
	}

	dir := t.TempDir()
	assert.Nil(t, os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte("# yarn\n"), 0600))
	assert.Nil(t, os.Chmod(filepath.Join(dir, "yarn.lock"), 0000))
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "yarn.lock"), 0600) })

	buffer := captureGitWarnings(t)
	options := files.LookupOptions{LookupDirectory: dir, RootDirectory: dir, GitBranch: "main", DefaultBranch: "main"}
	results := lookupAutomatic(options, []string{"node-modules", "nvm"})

	assert.Empty(t, results)
	output := buffer.String()
	assert.Contains(t, output, "Skipping node-modules cache: its lock file exists but could not be read.")
	assert.NotContains(t, output, "Skipping node-modules cache: no lock file")
	assert.Contains(t, output, "Skipping nvm cache: no lock file for it in this directory.")
}
