package storage

import (
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	assert "github.com/stretchr/testify/assert"
)

// silentListener accepts connections and never answers.
func silentListener(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	var conns []net.Conn
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			conns = append(conns, conn)
		}
	}()

	t.Cleanup(func() {
		_ = listener.Close()
		for _, conn := range conns {
			_ = conn.Close()
		}
	})

	return listener.Addr().String()
}

func newTestS3Storage(t *testing.T, url string) *S3Storage {
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

func Test__S3SilentEndpointFails(t *testing.T) {
	t.Setenv(HTTPTimeoutEnvVar, "1")
	storage := newTestS3Storage(t, "http://"+silentListener(t))

	file, _ := ioutil.TempFile(os.TempDir(), "*")
	_, _ = file.WriteString("content")
	_ = file.Close()
	defer os.Remove(file.Name())

	// 3 attempts of 1s plus at most 2 backoffs of 5s.
	bound := 15 * time.Second
	operations := map[string]func() error{
		"has_key": func() error { _, err := storage.HasKey("key"); return err },
		"store":   func() error { return storage.Store("key", file.Name()) },
		"restore": func() error {
			restored, err := storage.Restore("key")
			if restored != nil {
				_ = os.Remove(restored.Name())
			}
			return err
		},
		"list": func() error { _, err := storage.List(); return err },
	}

	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			start := time.Now()
			done := make(chan error, 1)
			go func() { done <- operation() }()

			select {
			case err := <-done:
				assert.Error(t, err)
				assert.Less(t, time.Since(start), bound)
			case <-time.After(bound):
				t.Fatalf("%s still hanging after %v", name, bound)
			}
		})
	}
}

func Test__S3SlowDownloadSucceeds(t *testing.T) {
	t.Setenv(HTTPTimeoutEnvVar, "1")
	content := strings.Repeat("x", 5*1024)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(content)-1, len(content)))
		w.WriteHeader(http.StatusPartialContent)

		// 5 chunks 600ms apart: 3s in total, longer than the 1s stall timeout.
		for i := 0; i < 5; i++ {
			time.Sleep(600 * time.Millisecond)
			_, _ = io.WriteString(w, content[i*1024:(i+1)*1024])
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()

	storage := newTestS3Storage(t, server.URL)
	restored, err := storage.Restore("key")
	if assert.NoError(t, err) {
		defer os.Remove(restored.Name())
		data, _ := ioutil.ReadFile(restored.Name())
		assert.Equal(t, content, string(data))
	}
}

func Test__StallConn(t *testing.T) {
	t.Run("write fails when the peer stops reading", func(t *testing.T) {
		client, server := net.Pipe()
		defer server.Close()
		conn := &stallConn{Conn: client, timeout: 300 * time.Millisecond}

		start := time.Now()
		_, err := conn.Write(make([]byte, 1024))
		assert.Error(t, err)
		assert.Less(t, time.Since(start), 2*time.Second)
	})

	t.Run("read fails when the peer sends nothing", func(t *testing.T) {
		client, server := net.Pipe()
		defer server.Close()
		conn := &stallConn{Conn: client, timeout: 300 * time.Millisecond}

		_, err := conn.Read(make([]byte, 1))
		netErr, ok := err.(net.Error)
		assert.True(t, ok && netErr.Timeout())
	})

	t.Run("slow but steady write succeeds", func(t *testing.T) {
		client, server := net.Pipe()
		conn := &stallConn{Conn: client, timeout: 300 * time.Millisecond}
		payload := make([]byte, 4*maxStallConnWriteSize)

		go func() {
			buffer := make([]byte, maxStallConnWriteSize)
			for {
				time.Sleep(150 * time.Millisecond)
				if _, err := server.Read(buffer); err != nil {
					return
				}
			}
		}()

		// 4 chunks 150ms apart: longer in total than the 300ms timeout.
		n, err := conn.Write(payload)
		assert.NoError(t, err)
		assert.Equal(t, len(payload), n)
		_ = server.Close()
	})
}

func Test__HTTPTimeoutsFromEnv(t *testing.T) {
	t.Setenv(HTTPTimeoutEnvVar, "")
	assert.Equal(t, httpTimeouts{Header: 30 * time.Second, Stall: 60 * time.Second}, httpTimeoutsFromEnv())

	t.Setenv(HTTPTimeoutEnvVar, "120")
	assert.Equal(t, httpTimeouts{Header: 120 * time.Second, Stall: 120 * time.Second}, httpTimeoutsFromEnv())

	for _, invalid := range []string{"abc", "0", "-5", "1.5"} {
		t.Setenv(HTTPTimeoutEnvVar, invalid)
		assert.Equal(t, httpTimeouts{Header: 30 * time.Second, Stall: 60 * time.Second}, httpTimeoutsFromEnv())
	}
}

func Test__SSHSilentServerFails(t *testing.T) {
	address := silentListener(t)
	config := &ssh.ClientConfig{
		User: "tester",
		// #nosec
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	start := time.Now()
	_, err := dialSSH(address, config, time.Second)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}
