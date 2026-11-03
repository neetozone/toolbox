package storage

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

// memoryS3 is a minimal path-style S3 that keeps objects in memory.
func memoryS3(t *testing.T) (*httptest.Server, map[string][]byte) {
	var mutex sync.Mutex
	objects := map[string][]byte{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()

		name := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch r.Method {
		case http.MethodPut:
			body, _ := ioutil.ReadAll(r.Body)
			objects[name] = body
		case http.MethodHead, http.MethodGet:
			body, ok := objects[name]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(body)))
			w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(body)-1, len(body)))
			if r.Method == http.MethodHead {
				return
			}

			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(body)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	}))

	t.Cleanup(server.Close)
	return server, objects
}

func memoryS3Storage(t *testing.T, url string) *S3Storage {
	storage, err := NewS3Storage(S3StorageOptions{
		URL:     url,
		Bucket:  "bucket",
		Project: "project",
		Config:  StorageConfig{MaxSpace: math.MaxInt64, SortKeysBy: SortByStoreTime},
	})

	if !assert.NoError(t, err) {
		t.FailNow()
	}

	return storage
}

func Test__S3LongKeys(t *testing.T) {
	file, _ := ioutil.TempFile(os.TempDir(), "*")
	_, _ = file.WriteString("long key content")
	_ = file.Close()
	defer os.Remove(file.Name())

	t.Run("stores and restores a 1000-character key", func(t *testing.T) {
		server, objects := memoryS3(t)
		storage := memoryS3Storage(t, server.URL)
		key := strings.Repeat("c", 1000)

		assert.NoError(t, storage.Store(key, file.Name()))
		assert.Contains(t, objects, "project/"+key)

		restored, err := storage.Restore(key)
		if assert.NoError(t, err) {
			defer os.Remove(restored.Name())
			assert.Less(t, len(filepath.Base(restored.Name())), 100)
			content, _ := ioutil.ReadFile(restored.Name())
			assert.Equal(t, "long key content", string(content))
		}
	})

	t.Run("accepts an object key of exactly 1024 bytes", func(t *testing.T) {
		server, objects := memoryS3(t)
		storage := memoryS3Storage(t, server.URL)
		key := strings.Repeat("d", MaxS3ObjectKeyBytes-len("project/"))

		assert.NoError(t, storage.Store(key, file.Name()))
		assert.Contains(t, objects, "project/"+key)
	})

	t.Run("rejects an object key over 1024 bytes before uploading", func(t *testing.T) {
		server, objects := memoryS3(t)
		storage := memoryS3Storage(t, server.URL)
		key := strings.Repeat("é", 510)

		err := storage.Store(key, file.Name())
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "cache key is too long")
			assert.Contains(t, err.Error(), "1028 bytes")
		}
		assert.Empty(t, objects)
	})

	t.Run("removes the temp file when a restore fails", func(t *testing.T) {
		server, _ := memoryS3(t)
		storage := memoryS3Storage(t, server.URL)
		key := "restore-failure-" + strings.Repeat("f", 20)
		pattern := filepath.Join(os.TempDir(), key+"-*")
		before, _ := filepath.Glob(pattern)

		_, err := storage.Restore(key)
		assert.Error(t, err)
		after, _ := filepath.Glob(pattern)
		assert.Equal(t, len(before), len(after))
	})
}
