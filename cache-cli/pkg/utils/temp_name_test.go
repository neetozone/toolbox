package utils

import (
	"strings"
	"testing"
	"unicode/utf8"

	assert "github.com/stretchr/testify/assert"
)

func Test__TempFilePrefix(t *testing.T) {
	t.Run("keeps short keys readable", func(t *testing.T) {
		prefix := TempFilePrefix("main-gems-abc")
		assert.True(t, strings.HasPrefix(prefix, "main-gems-abc-"))
		assert.Len(t, prefix, len("main-gems-abc-")+12)
	})

	t.Run("bounds long keys", func(t *testing.T) {
		prefix := TempFilePrefix(strings.Repeat("c", 1000))
		assert.Len(t, prefix, 64+1+12)
	})

	t.Run("distinguishes long keys sharing a prefix", func(t *testing.T) {
		long := strings.Repeat("c", 1000)
		assert.NotEqual(t, TempFilePrefix(long+"a"), TempFilePrefix(long+"b"))
	})

	t.Run("cuts multi-byte keys on a rune boundary", func(t *testing.T) {
		prefix := TempFilePrefix("a" + strings.Repeat("日", 100))
		assert.True(t, utf8.ValidString(prefix))
		assert.LessOrEqual(t, len(prefix), 64+1+12)
	})

	t.Run("removes path and pattern characters", func(t *testing.T) {
		prefix := TempFilePrefix(`a/b\c*d`)
		assert.True(t, strings.HasPrefix(prefix, "a-b-c-d-"))
	})
}
