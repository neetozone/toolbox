package archive

import (
	"os"
	"strconv"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
)

type Archiver interface {
	Compress(dst, src string) error
	Decompress(src string) (string, error)
}

func NewArchiver(metricsManager metrics.MetricsManager) Archiver {
	method := os.Getenv("NEETO_CI_CACHE_ARCHIVE_METHOD")
	switch method {
	case "native":
		return configure(NewNativeArchiver(metricsManager, false))
	case "native-parallel":
		return configure(NewNativeArchiver(metricsManager, true))
	case "native-zstd":
		archiver := NewNativeArchiver(metricsManager, true)
		archiver.UseZstd = true
		return configure(archiver)
	default:
		return NewShellOutArchiver(metricsManager)
	}
}

// configure applies the optional tuning env vars; unset keeps the defaults.
func configure(archiver *NativeArchiver) *NativeArchiver {
	archiver.UnpackWorkers = intFromEnv("NEETO_CI_CACHE_UNPACK_WORKERS", 1)
	archiver.CompressionLevel = intFromEnv("NEETO_CI_CACHE_COMPRESSION_LEVEL", 0)
	return archiver
}

func intFromEnv(name string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(name))
	if err != nil {
		return fallback
	}

	return value
}
