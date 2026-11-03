package cmd

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func useS3Backend(t *testing.T, url string) {
	t.Setenv("NEETO_CI_CACHE_BACKEND", "s3")
	t.Setenv("NEETO_CI_PROJECT_ID", "project")
	t.Setenv("NEETO_CI_CACHE_S3_BUCKET", "bucket")
	t.Setenv("NEETO_CI_CACHE_S3_URL", url)
}

func statusServer(t *testing.T, status int) string {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}))

	t.Cleanup(server.Close)
	return server.URL
}

func refusedURL(t *testing.T) string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}

	address := listener.Addr().String()
	_ = listener.Close()
	return "http://" + address
}

func Test__HasKeyExitCodes(t *testing.T) {
	t.Run("exists", func(t *testing.T) {
		useS3Backend(t, statusServer(t, http.StatusOK))
		assert.Equal(t, HasKeyExists, RunHasKey(hasKeyCmd, []string{"key"}))
	})

	t.Run("missing key", func(t *testing.T) {
		useS3Backend(t, statusServer(t, http.StatusNotFound))
		assert.Equal(t, HasKeyMissing, RunHasKey(hasKeyCmd, []string{"key"}))
	})

	for _, failOnError := range []string{"", "true"} {
		t.Run("forbidden with CACHE_FAIL_ON_ERROR="+failOnError, func(t *testing.T) {
			t.Setenv("CACHE_FAIL_ON_ERROR", failOnError)
			useS3Backend(t, statusServer(t, http.StatusForbidden))
			assert.Equal(t, HasKeyUnknown, RunHasKey(hasKeyCmd, []string{"key"}))
		})

		t.Run("connection refused with CACHE_FAIL_ON_ERROR="+failOnError, func(t *testing.T) {
			t.Setenv("CACHE_FAIL_ON_ERROR", failOnError)
			useS3Backend(t, refusedURL(t))
			assert.Equal(t, HasKeyUnknown, RunHasKey(hasKeyCmd, []string{"key"}))
		})
	}

	t.Run("server error", func(t *testing.T) {
		useS3Backend(t, statusServer(t, http.StatusInternalServerError))
		assert.Equal(t, HasKeyUnknown, RunHasKey(hasKeyCmd, []string{"key"}))
	})

	t.Run("no backend configured", func(t *testing.T) {
		t.Setenv("NEETO_CI_CACHE_BACKEND", "")
		assert.Equal(t, HasKeyUnknown, RunHasKey(hasKeyCmd, []string{"key"}))
	})

	t.Run("wrong number of arguments", func(t *testing.T) {
		assert.Equal(t, HasKeyUnknown, RunHasKey(hasKeyCmd, []string{}))
	})
}
