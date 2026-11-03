package storage

import (
	"bytes"
	"io/ioutil"
	"math"
	"path/filepath"
	"testing"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/logging"
	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func Test__S3ProfileMessageStaysOffStdout(t *testing.T) {
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	credentials := filepath.Join(dir, "credentials")
	_ = ioutil.WriteFile(config, []byte("[profile diagnostic-test]\nregion = us-east-1\n"), 0600)
	_ = ioutil.WriteFile(credentials, []byte("[diagnostic-test]\naws_access_key_id = AKIATEST\naws_secret_access_key = secret\n"), 0600)
	t.Setenv("AWS_CONFIG_FILE", config)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credentials)
	t.Setenv("NEETO_CI_CACHE_USE_EC2_INSTANCE_PROFILE", "")
	t.Setenv("NEETO_CI_CACHE_AWS_PROFILE", "diagnostic-test")

	var stdout, stderr bytes.Buffer
	previousOut, previousDiagnosticOut := log.StandardLogger().Out, logging.Diagnostic.Out
	log.SetOutput(&stdout)
	logging.Diagnostic.Out = &stderr
	defer func() {
		log.SetOutput(previousOut)
		logging.Diagnostic.Out = previousDiagnosticOut
	}()

	_, err := NewS3Storage(S3StorageOptions{
		Bucket:  "bucket",
		Project: "project",
		Config:  StorageConfig{MaxSpace: math.MaxInt64, SortKeysBy: SortByStoreTime},
	})

	assert.NoError(t, err)
	assert.Contains(t, stderr.String(), "Using 'diagnostic-test' AWS profile.")
	assert.NotContains(t, stdout.String(), "AWS profile")
}
