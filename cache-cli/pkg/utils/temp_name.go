package utils

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxTempNameKeyBytes = 64

// TempFilePrefix names a local temp file for a cache key, short and safe for any key length.
func TempFilePrefix(key string) string {
	readable := key
	if len(readable) > maxTempNameKeyBytes {
		readable = readable[:maxTempNameKeyBytes]
		for !utf8.ValidString(readable) {
			readable = readable[:len(readable)-1]
		}
	}

	readable = strings.NewReplacer("/", "-", "\\", "-", "*", "-").Replace(readable)
	return fmt.Sprintf("%s-%x", readable, sha256.Sum256([]byte(key)))[:len(readable)+13]
}
