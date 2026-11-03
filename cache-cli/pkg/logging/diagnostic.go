package logging

import (
	"os"

	log "github.com/sirupsen/logrus"
)

// Diagnostic writes informational messages to stderr, keeping stdout for command output.
var Diagnostic = &log.Logger{
	Out:       os.Stderr,
	Formatter: new(CustomFormatter),
	Hooks:     make(log.LevelHooks),
	Level:     log.InfoLevel,
}
