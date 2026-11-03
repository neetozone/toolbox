package archive

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
	assert "github.com/stretchr/testify/assert"
)

type treeEntry struct {
	mode  fs.FileMode
	size  int64
	sum   [32]byte
	link  string
	mtime int64
}

func buildTree(t *testing.T, root string) {
	rng := rand.New(rand.NewSource(7))
	mtime := time.Now().Add(-48 * time.Hour).Truncate(time.Second)

	write := func(path string, size int, mode fs.FileMode) {
		assert.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		content := make([]byte, size)
		rng.Read(content)
		assert.NoError(t, os.WriteFile(path, content, mode))
		assert.NoError(t, os.Chmod(path, mode))
		assert.NoError(t, os.Chtimes(path, mtime, mtime))
	}

	for i := 0; i < 300; i++ {
		write(filepath.Join(root, fmt.Sprintf("d%d", i%11), fmt.Sprintf("sub%d", i%5), fmt.Sprintf("f%d.rb", i)), rng.Intn(4096), 0644)
	}

	write(filepath.Join(root, "empty.txt"), 0, 0644)
	write(filepath.Join(root, "bin", "run.sh"), 100, 0755)
	write(filepath.Join(root, "ro", "locked.txt"), 50, 0444)
	write(filepath.Join(root, "big", "one.bin"), 2<<20, 0644)
	write(filepath.Join(root, "big", "two.bin"), (1<<20)+1, 0644)
	write(filepath.Join(root, "exact", "limit.bin"), 1<<20, 0644)

	assert.NoError(t, os.Symlink("../bin/run.sh", filepath.Join(root, "d0", "link-to-run")))
	assert.NoError(t, os.Symlink("big", filepath.Join(root, "link-to-dir")))

	// A read-only directory holding files: restored last by the archiver.
	write(filepath.Join(root, "rodir", "inside.txt"), 10, 0644)
	assert.NoError(t, os.Chmod(filepath.Join(root, "rodir"), 0555))
	t.Cleanup(func() { makeWritable(root) })
}

func makeWritable(root string) {
	_ = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			_ = os.Chmod(path, 0755)
		}

		return nil
	})
}

func snapshot(t *testing.T, root string) map[string]treeEntry {
	entries := map[string]treeEntry{}
	assert.NoError(t, filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		assert.NoError(t, err)
		rel, _ := filepath.Rel(root, path)
		entry := treeEntry{mode: info.Mode(), size: info.Size()}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			entry.link, _ = os.Readlink(path)
		case info.Mode().IsRegular():
			f, err := os.Open(path)
			assert.NoError(t, err)
			h := sha256.New()
			_, _ = io.Copy(h, f)
			_ = f.Close()
			copy(entry.sum[:], h.Sum(nil))
			entry.mtime = info.ModTime().Unix()
		default:
			entry.size = 0
		}

		entries[rel] = entry
		return nil
	}))

	return entries
}

func Test__NativeRoundTrip(t *testing.T) {
	newArchiver := func(parallelGzip, zstd bool, level, workers int) func() *NativeArchiver {
		return func() *NativeArchiver {
			return &NativeArchiver{
				MetricsManager:   metrics.NewNoOpMetricsManager(),
				UseParallelism:   parallelGzip,
				UseZstd:          zstd,
				CompressionLevel: level,
				UnpackWorkers:    workers,
			}
		}
	}

	gzipSeq := newArchiver(false, false, 0, 1)
	pgzipSeq := newArchiver(true, false, 0, 1)
	pgzipPar := newArchiver(true, false, 0, 8)
	pgzipFast := newArchiver(true, false, 1, 4)
	zstdSeq := newArchiver(false, true, 0, 1)
	zstdPar := newArchiver(false, true, 0, 8)

	// Each case compresses with `compress` and unpacks with `decompress`.
	cases := []struct {
		name       string
		compress   func() *NativeArchiver
		decompress func() *NativeArchiver
	}{
		{"gzip to gzip sequential", gzipSeq, gzipSeq},
		{"pgzip to parallel unpack", pgzipSeq, pgzipPar},
		{"gzip to parallel unpack", gzipSeq, pgzipPar},
		{"fast pgzip to parallel unpack", pgzipFast, pgzipPar},
		{"zstd to zstd sequential", zstdSeq, zstdSeq},
		{"zstd to zstd parallel unpack", zstdSeq, zstdPar},
		{"zstd read by a gzip configured archiver", zstdSeq, pgzipPar},
		{"gzip read by a zstd configured archiver", pgzipSeq, zstdPar},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parent := t.TempDir()
			root := filepath.Join(parent, "tree")
			buildTree(t, root)
			expected := snapshot(t, root)

			archivePath := filepath.Join(parent, "archive")
			assert.NoError(t, tc.compress().Compress(archivePath, root))

			// Remove the original so the unpack has to recreate it.
			makeWritable(root)
			assert.NoError(t, os.RemoveAll(root))

			_, err := tc.decompress().Decompress(archivePath)
			assert.NoError(t, err)

			actual := snapshot(t, root)
			assert.Equal(t, len(expected), len(actual))
			for path, want := range expected {
				got, ok := actual[path]
				if !assert.True(t, ok, "missing %s", path) {
					continue
				}

				assert.Equal(t, want.mode, got.mode, "mode of %s", path)
				assert.Equal(t, want.size, got.size, "size of %s", path)
				assert.Equal(t, want.sum, got.sum, "content of %s", path)
				assert.Equal(t, want.link, got.link, "link of %s", path)
				if want.mtime != 0 {
					assert.Equal(t, want.mtime, got.mtime, "mtime of %s", path)
				}
			}
		})
	}
}

func Test__DecompressRejectsUnknownFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-an-archive")
	assert.NoError(t, os.WriteFile(path, []byte("this is neither gzip nor zstd"), 0600))

	archiver := &NativeArchiver{MetricsManager: metrics.NewNoOpMetricsManager(), UseParallelism: true, UnpackWorkers: 4}
	_, err := archiver.Decompress(path)
	assert.Error(t, err)
}
