package storage

import (
	"context"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"syscall"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

// multipartS3 starts multipart uploads, answers parts with partStatus (0 blocks) and counts aborts.
type multipartS3 struct {
	partStatus  int
	partStarted chan struct{}
	once        sync.Once
	mutex       sync.Mutex
	aborts      []string
}

func (m *multipartS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	switch {
	case r.Method == http.MethodPost && query.Has("uploads"):
		_, _ = w.Write([]byte(`<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>project/key</Key><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`))
	case r.Method == http.MethodPut && query.Has("partNumber"):
		m.once.Do(func() { close(m.partStarted) })
		if m.partStatus == 0 {
			_, _ = io.Copy(ioutil.Discard, r.Body)
			<-r.Context().Done()
			return
		}

		w.WriteHeader(m.partStatus)
	case r.Method == http.MethodDelete && query.Has("uploadId"):
		m.mutex.Lock()
		m.aborts = append(m.aborts, query.Get("uploadId"))
		m.mutex.Unlock()
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

func (m *multipartS3) abortedUploads() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return append([]string{}, m.aborts...)
}

func multipartTestFile(t *testing.T) string {
	file, err := ioutil.TempFile(os.TempDir(), "*")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	// Larger than the uploader's 5 MB part size, so it uses a multipart upload.
	_, _ = file.Write(make([]byte, 6*1024*1024))
	_ = file.Close()
	t.Cleanup(func() { _ = os.Remove(file.Name()) })
	return file.Name()
}

func multipartTestStorage(t *testing.T, handler http.Handler) *S3Storage {
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	storage, err := NewS3Storage(S3StorageOptions{
		URL:     server.URL,
		Bucket:  "bucket",
		Project: "project",
		Config:  StorageConfig{MaxSpace: math.MaxInt64, SortKeysBy: SortByStoreTime},
	})

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return storage
}

func Test__S3InterruptedUploadIsAborted(t *testing.T) {
	fake := &multipartS3{partStarted: make(chan struct{})}
	storage := multipartTestStorage(t, fake)
	path := multipartTestFile(t)

	ctx, cancel := context.WithCancel(context.Background())
	previousContext, previousExit := uploadContext, exitProcess
	uploadContext = func() (context.Context, func() os.Signal) {
		return ctx, func() os.Signal { return syscall.SIGTERM }
	}
	exitCode := -1
	exitProcess = func(code int) { exitCode = code }
	defer func() { uploadContext, exitProcess = previousContext, previousExit }()

	go func() {
		<-fake.partStarted
		cancel()
	}()

	err := storage.Store("key", path)
	assert.Error(t, err)
	assert.Equal(t, []string{"upload-1"}, fake.abortedUploads())
	assert.Equal(t, 128+int(syscall.SIGTERM), exitCode)
}

func Test__S3FailedUploadIsAbortedOnce(t *testing.T) {
	fake := &multipartS3{partStatus: http.StatusForbidden, partStarted: make(chan struct{})}
	storage := multipartTestStorage(t, fake)

	err := storage.Store("key", multipartTestFile(t))
	assert.Error(t, err)
	assert.Equal(t, []string{"upload-1"}, fake.abortedUploads())
}
