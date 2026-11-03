//go:build !windows
// +build !windows

package storage

import (
	"os"
	"syscall"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
)

func Test__UploadContextIsCancelledBySignals(t *testing.T) {
	for _, signal := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		ctx, stop := uploadContext()
		assert.NoError(t, syscall.Kill(os.Getpid(), signal))

		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
			t.Fatalf("%v did not cancel the upload context", signal)
		}

		assert.Equal(t, signal, stop())
	}

	_, stop := uploadContext()
	assert.Nil(t, stop())
}
