package archive

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/klauspost/compress/zstd"
	pgzip "github.com/klauspost/pgzip"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

// Files up to this size are buffered and written by the unpack workers.
const maxBufferedFileSize = 1 << 20

var (
	gzipMagic = []byte{0x1f, 0x8b}
	zstdMagic = []byte{0x28, 0xb5, 0x2f, 0xfd}
)

type NativeArchiver struct {
	MetricsManager metrics.MetricsManager
	UseParallelism bool
	// UseZstd selects zstd for Compress. Decompress detects the format itself.
	UseZstd bool
	// CompressionLevel is codec specific; 0 keeps the codec default.
	CompressionLevel int
	// UnpackWorkers > 1 writes small files concurrently in Decompress.
	UnpackWorkers int
}

func NewNativeArchiver(metricsManager metrics.MetricsManager, useParallelism bool) *NativeArchiver {
	return &NativeArchiver{
		MetricsManager: metricsManager,
		UseParallelism: useParallelism,
		UnpackWorkers:  1,
	}
}

func (a *NativeArchiver) Compress(dst, src string) error {
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("error finding '%s': %v", src, err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_RDWR, os.FileMode(0644))
	if err != nil {
		return err
	}

	// The order is 'tar > compressor > file'
	gzipWriter, err := a.newCompressWriter(dstFile)
	if err != nil {
		return fmt.Errorf("error creating compressor: %v", err)
	}

	tarWriter := tar.NewWriter(gzipWriter)

	// We walk through every file in the specified path, adding them to the tar archive.
	err = filepath.Walk(src, func(fileName string, fileInfo os.FileInfo, e error) error {
		var link string
		if fileInfo.Mode()&os.ModeSymlink == os.ModeSymlink {
			if link, err = os.Readlink(fileName); err != nil {
				return fmt.Errorf("error reading symlink for '%s': %v", fileName, err)
			}
		}

		header, err := tar.FileInfoHeader(fileInfo, link)
		if err != nil {
			return fmt.Errorf("error creating tar header for '%s': %v", fileName, err)
		}

		// Truncate time to seconds only
		header.ModTime = header.ModTime.Truncate(time.Second)

		if fileInfo.IsDir() {
			header.Name = fileName + string(os.PathSeparator)
		} else {
			header.Name = fileName
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("error writing tar header: %v", err)
		}

		// If the file is not a regular file, nothing else to do for it
		if !fileInfo.Mode().IsRegular() {
			return nil
		}

		// If it is a regular file, we need to copy its contents to the archive
		file, err := os.Open(fileName)
		if err != nil {
			return fmt.Errorf("error opening file '%s': %v", fileName, err)
		}

		if _, err := io.Copy(tarWriter, file); err != nil {
			return fmt.Errorf("error writing file '%s' to tar archive: %v", fileName, err)
		}

		_ = file.Close()

		return nil
	})

	if err != nil {
		return fmt.Errorf("error walking tar archive: %v", err)
	}

	if err := tarWriter.Close(); err != nil {
		return fmt.Errorf("error closing tar writer: %v", err)
	}

	if err := gzipWriter.Close(); err != nil {
		return fmt.Errorf("error closing compressor: %v", err)
	}

	if err := dstFile.Close(); err != nil {
		return fmt.Errorf("error closing destination file '%s', %v", dst, err)
	}

	return nil
}

type directoryStat struct {
	name string
	mode fs.FileMode
}

func (a *NativeArchiver) Decompress(src string) (string, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("error opening '%s': %v", src, err)
	}

	defer srcFile.Close()

	uncompressedStream, err := a.newDecompressReader(srcFile)
	if err != nil {
		log.Errorf("error creating decompression reader: %v", err)
		a.publishCorruptionMetric()
		return "", err
	}

	defer uncompressedStream.Close()

	i := 0
	tarReader := tar.NewReader(uncompressedStream)
	restorationPath := ""
	var failures int32
	delayedDirectoryStats := []directoryStat{}
	markFailed := func() { atomic.StoreInt32(&failures, 1) }
	writer := newFileWriters(a.UnpackWorkers, markFailed)
	defer writer.finish()

	// We read all blocks in the tar archive.
	// If an error is found when processing a particular tar block,
	// it is logged, and we move to the next tar block.
	// The only reason we return an error, and stop execution,
	// is if we have issues reading the tar archive.
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			a.publishCorruptionMetric()
			return "", fmt.Errorf("error reading tar stream: %v", err)
		}

		// If it's the first file in archive, we keep track of its name.
		if i == 0 {
			restorationPath = header.Name
		}

		i++
		writer.waitIfPending(header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			mode := header.FileInfo().Mode()

			// Directories can be filled with files, but not be writable.
			// See: https://github.com/golang/go/issues/27161.
			// So, if we create the directory with the permissions on the tar header,
			// we are not able to create the files inside of it afterwards.
			// In those cases, we create the directory with 0770 permissions,
			// and delay setting the proper permissions on the directory after all files are extracted.
			if header.FileInfo().Mode()&0200 == 0 {
				mode = 0770
				delayedDirectoryStats = append(delayedDirectoryStats, directoryStat{
					name: header.Name,
					mode: header.FileInfo().Mode(),
				})
			}

			if err := os.MkdirAll(header.Name, mode); err != nil {
				log.Errorf("Error creating directory '%s': %v", header.Name, err)
				markFailed()
				continue
			}

		case tar.TypeSymlink:
			// we have to remove the symlink first, if it exists.
			// Otherwise os.Symlink will complain.
			if _, err := os.Lstat(header.Name); err == nil {
				_ = os.Remove(header.Name)
			}

			if err := os.Symlink(header.Linkname, header.Name); err != nil {
				log.Errorf("Error creating symlink '%s'-'%s': %v", header.Name, header.Linkname, err)
				markFailed()
				continue
			}

		case tar.TypeReg:
			if writer.concurrent() && header.Size <= maxBufferedFileSize {
				content := make([]byte, header.Size)
				if _, err := io.ReadFull(tarReader, content); err != nil {
					a.publishCorruptionMetric()
					writer.finish()
					return "", fmt.Errorf("error reading tar stream: %v", err)
				}

				writer.submit(a, header, content)
				continue
			}

			if err := a.writeRegularFile(header, tarReader); err != nil {
				log.Errorf("%v", err)
				markFailed()
				continue
			}
		}
	}

	writer.finish()

	for _, d := range delayedDirectoryStats {
		if err := os.Chmod(d.name, d.mode); err != nil {
			log.Errorf("error changing mode of directory '%s': %v", d.name, err)
			markFailed()
		}
	}

	if atomic.LoadInt32(&failures) != 0 {
		return restorationPath, fmt.Errorf("tar archive was not completely decompressed without errors")
	}

	return restorationPath, nil
}

func (a *NativeArchiver) openFile(header *tar.Header, tarReader *tar.Reader) (*os.File, error) {
	outFile, err := os.OpenFile(header.Name, os.O_RDWR|os.O_CREATE|os.O_EXCL, header.FileInfo().Mode())

	// File was opened successfully, just return it.
	if err == nil {
		return outFile, nil
	}

	// Since we are using O_EXCL, this error could mean that the file already exists.
	// If that is the case, we attempt to remove it before attempting to open it again.
	if errors.Is(err, os.ErrExist) {
		if err := os.Remove(header.Name); err != nil {
			return nil, fmt.Errorf("file '%s' already exists and can't be removed: %v", header.Name, err)
		}
	}

	// If a ErrNotExist is returned, it means the parent directory does not exist.
	// That means the file was included in the archive, but not its parent directory.
	// If that happens, we create the parent directory here, and try to open the file again.
	if errors.Is(err, os.ErrNotExist) {
		parentDir := filepath.Dir(header.Name)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, fmt.Errorf("error creating directory '%s' for '%s': %v", parentDir, header.Name, err)
		}
	}

	// Try to open file again now that we handled some possible errors.
	outFile, err = os.OpenFile(header.Name, os.O_RDWR|os.O_CREATE|os.O_EXCL, header.FileInfo().Mode())
	if err != nil {
		return nil, fmt.Errorf("error opening file '%s': %v", header.Name, err)
	}

	return outFile, nil
}

func (a *NativeArchiver) writeRegularFile(header *tar.Header, content io.Reader) error {
	outFile, err := a.openFile(header, nil)
	if err != nil {
		return fmt.Errorf("Error opening file '%s': %v", header.Name, err)
	}

	if _, err := io.Copy(outFile, content); err != nil {
		_ = outFile.Close()
		return fmt.Errorf("Error writing to file '%s': %v", header.Name, err)
	}

	if err := os.Chtimes(outFile.Name(), header.ModTime, header.ModTime); err != nil {
		_ = outFile.Close()
		return fmt.Errorf("Error changing timestamps for '%s': %v", header.Name, err)
	}

	if err := outFile.Close(); err != nil {
		return fmt.Errorf("Error closing file handle for '%s': %v", header.Name, err)
	}

	return nil
}

// fileWriters writes small regular files from a bounded pool of goroutines.
type fileWriters struct {
	jobs     chan fileJob
	inflight sync.WaitGroup
	pool     sync.WaitGroup
	pending  map[string]struct{}
	onError  func()
	once     sync.Once
}

type fileJob struct {
	archiver *NativeArchiver
	header   *tar.Header
	content  []byte
}

func newFileWriters(workers int, onError func()) *fileWriters {
	w := &fileWriters{onError: onError, pending: map[string]struct{}{}}
	if workers <= 1 {
		return w
	}

	w.jobs = make(chan fileJob, workers*4)
	for i := 0; i < workers; i++ {
		w.pool.Add(1)
		go func() {
			defer w.pool.Done()
			for job := range w.jobs {
				if err := job.archiver.writeRegularFile(job.header, bytes.NewReader(job.content)); err != nil {
					log.Errorf("%v", err)
					w.onError()
				}

				w.inflight.Done()
			}
		}()
	}

	return w
}

func (w *fileWriters) concurrent() bool { return w.jobs != nil }

func (w *fileWriters) submit(a *NativeArchiver, header *tar.Header, content []byte) {
	w.pending[header.Name] = struct{}{}
	w.inflight.Add(1)
	w.jobs <- fileJob{archiver: a, header: header, content: content}
}

// waitIfPending drains the workers when an entry reuses a path still being written.
func (w *fileWriters) waitIfPending(name string) {
	if _, ok := w.pending[name]; ok {
		w.inflight.Wait()
		w.pending = map[string]struct{}{}
	}
}

func (w *fileWriters) finish() {
	w.once.Do(func() {
		if w.jobs != nil {
			close(w.jobs)
			w.pool.Wait()
		}
	})
}

func (a *NativeArchiver) newCompressWriter(dstFile *os.File) (io.WriteCloser, error) {
	if a.UseZstd {
		level := zstd.SpeedDefault
		if a.CompressionLevel != 0 {
			level = zstd.EncoderLevelFromZstd(a.CompressionLevel)
		}

		return zstd.NewWriter(dstFile, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(runtime.GOMAXPROCS(0)))
	}

	level := gzip.DefaultCompression
	if a.CompressionLevel != 0 {
		level = a.CompressionLevel
	}

	if a.UseParallelism {
		return pgzip.NewWriterLevel(dstFile, level)
	}

	return gzip.NewWriterLevel(dstFile, level)
}

// newDecompressReader picks gzip or zstd from the magic bytes, whatever the configured method.
func (a *NativeArchiver) newDecompressReader(srcFile *os.File) (io.ReadCloser, error) {
	buffered := bufio.NewReaderSize(srcFile, 1<<20)
	magic, _ := buffered.Peek(len(zstdMagic))

	if bytes.HasPrefix(magic, zstdMagic) {
		decoder, err := zstd.NewReader(buffered, zstd.WithDecoderConcurrency(runtime.GOMAXPROCS(0)))
		if err != nil {
			return nil, err
		}

		return decoder.IOReadCloser(), nil
	}

	if a.UseParallelism {
		return pgzip.NewReader(buffered)
	}

	return gzip.NewReader(buffered)
}

func (a *NativeArchiver) publishCorruptionMetric() {
	err := a.MetricsManager.Publish(metrics.Metric{Name: metrics.CacheCorruptionRate, Value: "1"})
	if err != nil {
		log.Errorf("error publishing %s metric: %v", metrics.CacheCorruptionRate, err)
	}
}
