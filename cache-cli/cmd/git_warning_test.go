package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func captureGitWarnings(t *testing.T) *bytes.Buffer {
	var buffer bytes.Buffer
	previous := log.StandardLogger().Out
	previousFormatter := log.StandardLogger().Formatter
	log.SetOutput(&buffer)
	// Other tests install a formatter that drops the level, so set our own.
	log.SetFormatter(&log.TextFormatter{DisableColors: true, DisableTimestamp: true})
	gitWarningOnce = sync.Once{}
	t.Cleanup(func() {
		log.SetOutput(previous)
		log.SetFormatter(previousFormatter)
		gitWarningOnce = sync.Once{}
	})

	return &buffer
}

func chdirForTest(t *testing.T, dir string) {
	previous, err := os.Getwd()
	assert.Nil(t, err)
	assert.Nil(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

func Test__RunGitWarnsOutsideRepository(t *testing.T) {
	buffer := captureGitWarnings(t)
	chdirForTest(t, t.TempDir())
	t.Setenv("GIT_CEILING_DIRECTORIES", os.TempDir())
	t.Setenv("GIT_DIR", "")
	os.Unsetenv("GIT_DIR")

	assert.Equal(t, "", FindGitRoot())
	assert.Equal(t, "", FindGitRoot())

	output := buffer.String()
	assert.Equal(t, 1, strings.Count(output, "level=warning"))
	assert.Contains(t, output, "git rev-parse --show-toplevel failed: fatal: not a git repository")
	assert.Contains(t, output, "automatic cache keys are not scoped by directory")
	assert.NotContains(t, output, "owned by the user")
}

func Test__RunGitWarnsOnDubiousOwnership(t *testing.T) {
	buffer := captureGitWarnings(t)
	stubDir := t.TempDir()
	stub := "#!/bin/sh\n" +
		"echo \"fatal: detected dubious ownership in repository at '/repo'\" >&2\n" +
		"echo \"To add an exception for this directory, call:\" >&2\n" +
		"exit 128\n"
	assert.Nil(t, os.WriteFile(filepath.Join(stubDir, "git"), []byte(stub), 0700))
	t.Setenv("PATH", stubDir)
	t.Setenv("NEETO_CI_GIT_BRANCH", "")
	t.Setenv("NEETO_CI_GIT_PR_BRANCH", "")

	assert.Equal(t, "", FindGitBranch())
	assert.Equal(t, "", FindGitRoot())

	output := buffer.String()
	assert.Equal(t, 1, strings.Count(output, "level=warning"))
	assert.Contains(t, output, "git rev-parse --abbrev-ref HEAD failed: fatal: detected dubious ownership in repository at '/repo'")
	assert.Contains(t, output, "make the checkout owned by the user running 'cache'")
	assert.NotContains(t, output, "To add an exception")
}
