package archive

// Benchmarks for the unpack/compress options. Run on the CI instance type:
//
//	CACHE_BENCH_CORPUS=/path/to/vendor/bundle GOMAXPROCS=4 \
//	  go test ./pkg/archive -run '^$' -bench . -benchtime 5x -benchmem
//
// The corpus is copied to a temp dir first and only the copy is deleted.
// Without CACHE_BENCH_CORPUS a small synthetic tree is used (smoke test only).

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
)

func benchArchiver(parallelGzip, zstd bool, level, workers int) *NativeArchiver {
	return &NativeArchiver{
		MetricsManager:   metrics.NewNoOpMetricsManager(),
		UseParallelism:   parallelGzip,
		UseZstd:          zstd,
		CompressionLevel: level,
		UnpackWorkers:    workers,
	}
}

// benchCorpus returns a private copy of the corpus and its size in bytes.
func benchCorpus(b *testing.B) (string, int64) {
	root := filepath.Join(b.TempDir(), "corpus")
	if source := os.Getenv("CACHE_BENCH_CORPUS"); source != "" {
		copyTree(b, source, root)
	} else {
		syntheticTree(b, root)
	}

	var size int64
	_ = filepath.Walk(root, func(_ string, info fs.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			size += info.Size()
		}

		return nil
	})

	b.Logf("corpus %s: %.0f MB, GOMAXPROCS=%d, NumCPU=%d", root, float64(size)/1e6, runtime.GOMAXPROCS(0), runtime.NumCPU())
	return root, size
}

func syntheticTree(b *testing.B, root string) {
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 4000; i++ {
		path := filepath.Join(root, fmt.Sprintf("gem%d", i%200), "lib", fmt.Sprintf("dir%d", i%7), fmt.Sprintf("file%d.rb", i))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			b.Fatal(err)
		}

		// Text-like content so the compression ratio is not meaningless.
		content := make([]byte, 200+rng.Intn(12000))
		for j := range content {
			content[j] = "def end class module self nil true false @ivar\n  "[rng.Intn(48)]
		}

		if err := os.WriteFile(path, content, 0644); err != nil {
			b.Fatal(err)
		}
	}
}

func copyTree(b *testing.B, source, destination string) {
	err := filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(source, path)
		target := filepath.Join(destination, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0755)
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}

			return os.Symlink(link, target)
		case info.Mode().IsRegular():
			in, err := os.Open(path)
			if err != nil {
				return err
			}

			defer in.Close()
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, info.Mode().Perm()|0200)
			if err != nil {
				return err
			}

			if _, err := io.Copy(out, in); err != nil {
				_ = out.Close()
				return err
			}

			if err := out.Close(); err != nil {
				return err
			}

			return os.Chtimes(target, info.ModTime(), info.ModTime())
		}

		return nil
	})
	if err != nil {
		b.Fatalf("copying corpus: %v", err)
	}
}

func BenchmarkCompress(b *testing.B) {
	root, size := benchCorpus(b)
	cases := []struct {
		name     string
		archiver *NativeArchiver
	}{
		{"gzip-default", benchArchiver(false, false, 0, 1)},
		{"pgzip-default", benchArchiver(true, false, 0, 1)},
		{"pgzip-level1", benchArchiver(true, false, 1, 1)},
		{"pgzip-level6", benchArchiver(true, false, 6, 1)},
		{"zstd-level1", benchArchiver(false, true, 1, 1)},
		{"zstd-level3", benchArchiver(false, true, 3, 1)},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			dst := filepath.Join(b.TempDir(), "archive")
			b.SetBytes(size)
			var archived int64
			for i := 0; i < b.N; i++ {
				_ = os.Remove(dst)
				if err := tc.archiver.Compress(dst, root); err != nil {
					b.Fatal(err)
				}

				info, _ := os.Stat(dst)
				archived = info.Size()
			}

			b.ReportMetric(float64(archived)/float64(size), "ratio")
			b.ReportMetric(float64(archived)/1e6, "archive-MB")
		})
	}
}

func BenchmarkDecompress(b *testing.B) {
	root, size := benchCorpus(b)
	codecs := []struct {
		name     string
		compress *NativeArchiver
	}{
		{"pgzip", benchArchiver(true, false, 0, 1)},
		{"zstd", benchArchiver(false, true, 0, 1)},
	}

	for _, codec := range codecs {
		archivePath := filepath.Join(b.TempDir(), "archive-"+codec.name)
		if err := codec.compress.Compress(archivePath, root); err != nil {
			b.Fatal(err)
		}

		for _, workers := range []int{1, 2, 4, 8} {
			b.Run(fmt.Sprintf("%s/workers=%d", codec.name, workers), func(b *testing.B) {
				archiver := benchArchiver(true, false, 0, workers)
				b.SetBytes(size)
				for i := 0; i < b.N; i++ {
					b.StopTimer()
					makeWritable(root)
					if err := os.RemoveAll(root); err != nil {
						b.Fatal(err)
					}

					b.StartTimer()
					if _, err := archiver.Decompress(archivePath); err != nil {
						b.Fatal(err)
					}
				}
			})
		}

		// Reads and decompresses the stream but writes nothing: the floor the
		// file writes are measured against.
		b.Run(codec.name+"/no-writes", func(b *testing.B) {
			archiver := benchArchiver(true, false, 0, 1)
			b.SetBytes(size)
			for i := 0; i < b.N; i++ {
				f, err := os.Open(archivePath)
				if err != nil {
					b.Fatal(err)
				}

				stream, err := archiver.newDecompressReader(f)
				if err != nil {
					b.Fatal(err)
				}

				reader := tar.NewReader(stream)
				for {
					if _, err := reader.Next(); err == io.EOF {
						break
					} else if err != nil {
						b.Fatal(err)
					}

					_, _ = io.Copy(io.Discard, reader)
				}

				_ = stream.Close()
				_ = f.Close()
			}
		})
	}
}
