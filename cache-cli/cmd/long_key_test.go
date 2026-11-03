package cmd

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/archive"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
	assert "github.com/stretchr/testify/assert"
)

func Test__CompressLongKey(t *testing.T) {
	dir, _ := ioutil.TempDir(os.TempDir(), "*")
	defer os.RemoveAll(dir)
	_ = ioutil.WriteFile(filepath.Join(dir, "file"), []byte("content"), 0600)

	archiver := archive.NewArchiver(metrics.NewNoOpMetricsManager())
	dst, size, err := compress(archiver, strings.Repeat("c", 1000), dir)
	if assert.NoError(t, err) {
		defer os.Remove(dst)
		assert.Greater(t, size, int64(0))
		assert.Less(t, len(filepath.Base(dst)), 100)
	}
}
